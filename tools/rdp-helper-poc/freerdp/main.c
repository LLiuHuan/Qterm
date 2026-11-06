// qterm-freerdp-helper 是无窗口 FreeRDP 技术探针。
// stdin 先接收每行一个 FreeRDP 参数，以空行结束；之后接收固定 24 字节输入事件
// （个别事件类型会在 24 字节之后紧跟一段负载，见下面的 INPUT_CLIPBOARD_SET）。
// stdout 只输出与 IronRDP helper 相同的 44 字节帧头和 RGBA 脏矩形。
// stderr 承载全部日志，以及上层按行解析的机器可读契约行：
//   QTERM-CERT-*（证书指纹固定，语义已冻结）
//   QTERM-CLIPBOARD-B64 <base64(UTF-8 文本)> / QTERM-CLIPBOARD-EMPTY（远端剪贴板文本）
//   QTERM-RESIZE-UNSUPPORTED（动态分辨率请求无法完成；每次尺寸请求最多一行）
//   QTERM-RDP-FALLBACK no-frames（动态分辨率开着却连接后 4 秒内一帧都没有；随后干净退出，退出码 6）
//
// 动态分辨率（输入事件 9）**默认开启**，QTERM_RDP_DYNAMIC_RESOLUTION=0 可关闭；
// 开启时若"连上但一帧都没有"会自动降级退出（退出码 6），由上层记住该主机并改用 =0 重连一次。
// 详见 DYNAMIC_RESOLUTION_ENV / DYNAMIC_RESOLUTION_FALLBACK_MS 常量处的说明。
// 关闭时收到 type=9 仍按契约回一行 QTERM-RESIZE-UNSUPPORTED（上层据此走重连回退）。
//
// 证书策略：本 helper 只做「证书指纹固定（TOFU 的强校验部分）」。
// 首次连接由上层（Go）把呈送的叶子证书 SHA-256 展示给用户并落库，之后每次连接通过环境变量
// QTERM_RDP_CERT_SHA256 把已信任指纹传进来；helper 重新计算本次证书指纹并比对。
// 旧的 /cert:ignore（静默接受任何证书）已移除，未信任的证书一律拒绝，
// 把「是否信任」交回上层询问用户。

#include <ctype.h>
#include <errno.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#ifndef _WIN32
#include <poll.h>
#endif

#include <freerdp/channels/cliprdr.h>
#include <freerdp/client.h>
#include <freerdp/client/cliprdr.h>
#include <freerdp/client/cmdline.h>
#include <freerdp/client/disp.h>
#include <freerdp/codec/color.h>
#include <freerdp/crypto/certificate.h>
#include <freerdp/crypto/crypto.h>
#include <freerdp/event.h>
#include <freerdp/freerdp.h>
#include <freerdp/gdi/gdi.h>
#include <freerdp/input.h>
#include <winpr/collections.h>
#include <winpr/crypto.h>
#include <winpr/file.h>
#include <winpr/interlocked.h>
#include <winpr/string.h>
#include <winpr/synch.h>
#include <winpr/sysinfo.h>
#include <winpr/thread.h>
#include <winpr/user.h>
#include <winpr/wlog.h>

#define FRAME_HEADER_BYTES 44
#define INPUT_BYTES 24
#define INPUT_MOUSE_MOVE 1
#define INPUT_MOUSE_BUTTON 2
#define INPUT_WHEEL 3
#define INPUT_KEY 4
#define INPUT_CLOSE 5
// type=6：Unicode 键盘事件，value 低 16 位 = UTF-16 码元，flags bit0 = 按下/抬起。
#define INPUT_UNICODE_KEY 6
// type=7：设置「本机剪贴板文本」，value = 紧跟其后、尚未读取的 UTF-8 字节数。
#define INPUT_CLIPBOARD_SET 7
// type=8：请求立刻把当前远端剪贴板文本再上报一次（界面手动刷新用）。
#define INPUT_CLIPBOARD_RESEND 8
// type=9：请求把远端桌面改成 x（宽）× y（高），不重连、不重建会话。
// 与 type=7 不同，它**没有**尾部负载；value/flags 忽略。语义与取值范围已冻结。
#define INPUT_RESIZE 9

// 动态分辨率的合法范围（与上层冻结）：640x480 ~ 7680x4320。
// 越界值只打一行警告并忽略事件，绝不改变事件流解析，也绝不让 helper 崩掉。
#define RESIZE_MIN_WIDTH 640U
#define RESIZE_MIN_HEIGHT 480U
#define RESIZE_MAX_WIDTH 7680U
#define RESIZE_MAX_HEIGHT 4320U

// 发出 monitor layout 请求后等待服务端"重新协商尺寸"的宽限时间。
// 为什么需要它：服务端可能协商了 disp 通道、SendMonitorLayout 也返回成功，但实际忽略了请求
// （旧版本 / 组策略禁止 / 会话已在别处被改过尺寸），只靠返回值分辨不出来；
// 超时仍未收到 DesktopResize 回调就上报 QTERM-RESIZE-UNSUPPORTED，但**不重连、不阻塞出帧**。
#define RESIZE_ACK_TIMEOUT_MS 10000U

// 本机剪贴板文本的长度上限。上层一旦送出超限的事件，helper 必须把声明的字节数**读干净再丢弃**，
// 否则剩余字节会被当成下一个事件的头，事件流从此永久错位。
#define CLIPBOARD_TEXT_MAX_BYTES (1024U * 1024U)

// 退出兜底看门狗：每秒检查一次父进程/stdin（不空转），异常后最多再宽限 3 秒就强制退出。
#define HELPER_WATCHDOG_INTERVAL_MS 1000
#define HELPER_WATCHDOG_GRACE_MS 3000

// Windows 标准剪贴板格式号（与 winpr/user.h 的 CF_* 对应，这里只用到两个文本格式）。
#define CLIPRDR_FORMAT_CF_TEXT 1U
#define CLIPRDR_FORMAT_CF_UNICODETEXT 13U

// 证书指纹固定相关常量与退出码。
#define CERT_SHA256_ENV "QTERM_RDP_CERT_SHA256"
#define CERT_INSECURE_ENV "QTERM_RDP_CERT_INSECURE"
#define CERT_SHA256_BYTES 32
#define CERT_FINGERPRINT_CHARS (CERT_SHA256_BYTES * 2)
#define CERT_FINGERPRINT_INPUT_CHARS 256
// 因证书未信任/不匹配而中止连接时的稳定退出码，上层据此区分「等待用户决定」与其它失败。
#define EXIT_CERTIFICATE_REJECTED 5

// 动态分辨率（事件 9）的开关，**默认开启**：未设置或非 "0" 都启用，只有显式
// QTERM_RDP_DYNAMIC_RESOLUTION=0 才关闭。
//
// 为什么要"默认开 + 自动降级"（真机回归，2026-10-14）：现代目标 100.66.1.3
// （Windows，CN=DESKTOP-9PN7QMV）上开启 disp 后出现过「证书正常、连接成功，但累计 0 帧」，
// 而同一个二进制在 2008 R2 上出帧正常。为了既保住这个能力、又不把用户卡在黑屏上：
//   1) 默认开启（绝大多数目标走这条正常路）；
//   2) 开启后若连接完成起 4 秒内一帧都没有 → 打一行 QTERM-RDP-FALLBACK no-frames 并**干净退出**
//      （退出码 6 = EXIT_DYNAMIC_RESOLUTION_FALLBACK），由上层记住"这台主机别开动态分辨率"，
//      下次连接带上 QTERM_RDP_DYNAMIC_RESOLUTION=0（用户只需自动重连一次）。
#define DYNAMIC_RESOLUTION_ENV "QTERM_RDP_DYNAMIC_RESOLUTION"
// "开了反而没画面"的判定窗口：从 PostConnect 起算，窗口内没有任何帧就降级退出。
#define DYNAMIC_RESOLUTION_FALLBACK_MS 4000U
// 动态分辨率自我降级的稳定退出码，上层据此与普通失败区分（0=正常，5=证书被拒）。
#define EXIT_DYNAMIC_RESOLUTION_FALLBACK 6

// 剪贴板方向/内容（由上层按主机设置下发；两个变量都由 Go 侧 helperEnvironment 显式覆盖）。
//
//   QTERM_RDP_CLIPBOARD_MODE    off / both（默认）/ local-to-remote / remote-to-local
//   QTERM_RDP_CLIPBOARD_CONTENT text（默认）/ text-and-files
//
// 方向与 FreeRDP 的 ClipboardFeatureMask 逐位对应（见下面的 CLIPBOARD_FLAG_*）；
// off 的语义是"连 CLIPRDR 通道都不挂"（RedirectClipboard=FALSE），而不是只关掉我们自己的回调——
// 通道还在的话，本机与远端仍会互发剪贴板，用户以为关了其实没关。
#define CLIPBOARD_MODE_ENV "QTERM_RDP_CLIPBOARD_MODE"
#define CLIPBOARD_CONTENT_ENV "QTERM_RDP_CLIPBOARD_CONTENT"

// 与 include/freerdp/settings_types.h 的 CLIPRDR_FLAG_* 逐位对齐（改一处必须核对另一处）。
#define CLIPBOARD_FLAG_LOCAL_TO_REMOTE 0x01U
#define CLIPBOARD_FLAG_LOCAL_TO_REMOTE_FILES 0x02U
#define CLIPBOARD_FLAG_REMOTE_TO_LOCAL 0x10U
#define CLIPBOARD_FLAG_REMOTE_TO_LOCAL_FILES 0x20U

// 文件剪贴板（*_FILES 位）当前**永不置位**：本机侧的文件剪贴板读写与 FILEDESCRIPTOR/FILECONTENTS
// 处理都还没实现，位一旦置位就等于向对端声明"我们能收发文件"，而用户复制文件过去不会有任何反应。
// 上层（internal/remotedesktop/clipboard.go）按同一口径只下发文本能力；这里再挡一道，
// 保证手工设了 QTERM_RDP_CLIPBOARD_CONTENT=text-and-files 也不会产生假能力声明。
#define CLIPBOARD_FILE_TRANSFER_IMPLEMENTED 0

typedef struct
{
    rdpClientContext common;
    uint64_t sequence;
    BOOL output_failed;

    // ---- 剪贴板状态 ----
    // cliprdr 通道连上后由 ChannelConnected 事件填入；为空表示本会话没有剪贴板通道
    // （也可能是 QTERM_RDP_CLIPBOARD_MODE=off 时我们主动没挂）。
    CliprdrClientContext* cliprdr;
    // 剪贴板能力位（FreeRDP ClipboardFeatureMask）；0 = 关闭（连通道都不挂）。
    UINT32 clipboard_feature_mask;
    // 供启动诊断行使用，不参与协议。
    const char* clipboard_mode_label;
    const char* clipboard_content_label;

    // 「本机剪贴板文本」（事件 7 设置，UTF-8 字节，不保证以 '\0' 结尾）。
    // 只放在内存里：剪贴板内容可能包含用户复制的口令等敏感信息，绝不落盘、绝不进日志明文。
    BYTE* local_text;
    size_t local_length;
    // 是否已被事件 7 设置过。区分「本机剪贴板为空」与「上层还没同步过剪贴板」，
    // 前者在服务端请求粘贴时应答 CB_RESPONSE_FAIL，后者不应答空串（避免把空文本塞进对方剪贴板）。
    BOOL local_ready;

    // 最近一次上报给上层的远端剪贴板状态，用于去重：远端没变化时不重复打印契约行。
    char* remote_text;
    size_t remote_length;
    BOOL remote_report_valid;
    BOOL remote_report_empty;

    // 远端格式清单里我们选中的文本格式号（0 = 清单里没有文本格式）。
    UINT32 remote_request_format;
    // 是否已经发出 CB_FORMAT_DATA_REQUEST 正在等回应；回应里没有格式号，只能靠它判断数据格式。
    BOOL remote_request_pending;

    // ---- 动态分辨率（disp / Display Control 虚拟通道）----
    // 是否启用（默认启用；QTERM_RDP_DYNAMIC_RESOLUTION=0 关闭）。
    BOOL dynamic_resolution_enabled;
    // 通道连上后由 ChannelConnected 事件填入；NULL = 本次会话没有该通道（服务端不支持）。
    DispClientContext* disp;
    // 服务端是否已经发来 DisplayControlCaps（RDPEDISP 的能力 PDU）。
    // 只有这一步完成才说明服务端真的实现了该通道，之前发 monitor layout 属于未协商行为。
    BOOL disp_activated;
    // 已经发出、但还没在 DesktopResize 回调里看到回应的尺寸请求。
    // 主循环按 resize_pending_deadline 判超时，保证"服务端忽略请求"也能被上报且只上报一次。
    BOOL resize_pending;
    UINT32 resize_pending_width;
    UINT32 resize_pending_height;
    UINT64 resize_pending_deadline;

    // ---- "开了动态分辨率反而没画面"的自我降级 ----
    // 每个真正写出去的帧 +1（helper_end_paint 成功路径）。只认它，不认日志。
    uint64_t frames_output;
    // 启用动态分辨率时才武装：PostConnect 记起点，窗口内 frames_output 仍为 0 → 降级退出。
    BOOL fallback_armed;
    // 已经因为"没有帧"降级退出（决定进程退出码）。
    BOOL fallback_fired;
    UINT64 fallback_deadline;
} helperContext;

static uint32_t read_u32_le(const BYTE* data)
{
    return (uint32_t)data[0] | ((uint32_t)data[1] << 8U) | ((uint32_t)data[2] << 16U) |
           ((uint32_t)data[3] << 24U);
}

static void write_u32_le(BYTE* data, uint32_t value)
{
    data[0] = (BYTE)value;
    data[1] = (BYTE)(value >> 8U);
    data[2] = (BYTE)(value >> 16U);
    data[3] = (BYTE)(value >> 24U);
}

static void write_u64_le(BYTE* data, uint64_t value)
{
    write_u32_le(data, (uint32_t)value);
    write_u32_le(data + 4, (uint32_t)(value >> 32U));
}

static BOOL write_all(const void* data, size_t length)
{
    return fwrite(data, 1, length, stdout) == length;
}

// ---- 证书指纹固定 ----

// FreeRDP 在「未知证书」与「证书变更」两条路径上都可能调用校验回调，
// 同一次连接（乃至同一个进程）因此可能被回调多次；而上层是按行解析 stderr 的，
// 重复的判定行会被误当成多次事件，所以这里保证一组行只输出一次。
static BOOL certificate_reported = FALSE;
static BOOL certificate_insecure_reported = FALSE;
// 记录本次连接是否被证书校验拒绝：freerdp_get_last_error 在证书失败时可能返回 0，
// 单独存一个标记才能保证进程以稳定的非零码退出。
static BOOL certificate_rejected = FALSE;

static void cert_write_hex(const BYTE* data, size_t length, char* output)
{
    static const char digits[] = "0123456789abcdef";
    for (size_t index = 0; index < length; index++)
    {
        output[index * 2] = digits[(data[index] >> 4) & 0x0f];
        output[index * 2 + 1] = digits[data[index] & 0x0f];
    }
    output[length * 2] = '\0';
}

// 契约要求主题/签发者是单行：X509 名称打印虽然用 ONELINE，但个别证书仍可能带控制字符，
// 这里统一压成空格，避免上层按行解析时把一行拆成两行。
static void cert_flatten_line(char* text)
{
    if (!text)
        return;
    for (char* cursor = text; *cursor != '\0'; cursor++)
    {
        if (*cursor == '\r' || *cursor == '\n')
            *cursor = ' ';
    }
}

// 比较口径（已冻结）：大小写不敏感，并且允许任一侧用冒号分隔（两种常见指纹写法都能对上），
// 顺带忽略空白字符，避免上层写环境变量时带上换行/空格。
static void cert_normalize_fingerprint(const char* input, char* output, size_t capacity)
{
    size_t written = 0;
    if (!input)
    {
        output[0] = '\0';
        return;
    }
    for (const char* cursor = input; (*cursor != '\0') && (written + 1 < capacity); cursor++)
    {
        const unsigned char current = (unsigned char)*cursor;
        if ((current == ':') || isspace(current))
            continue;
        output[written] = (char)tolower(current);
        written++;
    }
    output[written] = '\0';
}

// FreeRDP 3 的 VerifyX509Certificate 回调：data 是**完整证书链的 PEM**（叶子证书在最前），
// length 不含结尾的 '\0'。返回 1 = 接受，0 = 拒绝（FreeRDP 会把 TLS 握手判为失败）。
static int helper_verify_x509_certificate(freerdp* instance, const BYTE* data, size_t length,
                                          const char* hostname, UINT16 port, DWORD flags)
{
    (void)instance;
    (void)hostname;
    (void)port;
    (void)flags;

    char fingerprint[CERT_FINGERPRINT_CHARS + 1] = { 0 };
    char* subject = NULL;
    char* issuer = NULL;

    // data 不保证以 '\0' 结尾，而证书解析按 C 字符串读取，因此先复制成带终止符的缓冲区。
    if (data && (length > 0))
    {
        char* pem = calloc(length + 1, sizeof(char));
        if (pem)
        {
            memcpy(pem, data, length);
            rdpCertificate* certificate = freerdp_certificate_new_from_pem(pem);
            if (certificate)
            {
                // 对「呈送的叶子证书 DER」求 SHA-256：freerdp_certificate_get_der 给的就是
                // 叶子证书的 DER 编码，winpr_Digest 直接算摘要，再由本文件十六进制小写化。
                size_t der_length = 0;
                BYTE* der = freerdp_certificate_get_der(certificate, &der_length);
                BYTE digest[CERT_SHA256_BYTES] = { 0 };
                if (der && (der_length > 0) &&
                    winpr_Digest(WINPR_MD_SHA256, der, der_length, digest, sizeof(digest)))
                    cert_write_hex(digest, sizeof(digest), fingerprint);
                free(der);

                // 主题/签发者尽力而为：解析失败就保持空串（契约允许）。
                subject = freerdp_certificate_get_subject(certificate);
                issuer = freerdp_certificate_get_issuer(certificate);
                cert_flatten_line(subject);
                cert_flatten_line(issuer);

                freerdp_certificate_free(certificate);
            }
            free(pem);
        }
    }

    // 无论接受还是拒绝都先输出这三行，且整组只输出一次；stdout 绝不参与（帧通道）。
    if (!certificate_reported)
    {
        certificate_reported = TRUE;
        fprintf(stderr, "QTERM-CERT-SHA256 %s\n", fingerprint);
        fprintf(stderr, "QTERM-CERT-SUBJECT %s\n", subject ? subject : "");
        fprintf(stderr, "QTERM-CERT-ISSUER %s\n", issuer ? issuer : "");
        fflush(stderr);
    }

    char expected[CERT_FINGERPRINT_INPUT_CHARS] = { 0 };
    cert_normalize_fingerprint(getenv(CERT_SHA256_ENV), expected, sizeof(expected));

    char actual[CERT_FINGERPRINT_CHARS + 1] = { 0 };
    cert_normalize_fingerprint(fingerprint, actual, sizeof(actual));

    int decision = 0;
    const char* insecure = getenv(CERT_INSECURE_ENV);
    if (insecure && (strcmp(insecure, "1") == 0))
    {
        // 排障专用：QTERM_RDP_CERT_INSECURE=1 时回到旧的「忽略证书」行为。
        // 这只用于定位握手问题，绝不能让上层在正常流程里设置它。
        if (!certificate_insecure_reported)
        {
            certificate_insecure_reported = TRUE;
            fprintf(stderr, "QTERM-CERT-INSECURE\n");
            fflush(stderr);
        }
        decision = 1;
    }
    else if (expected[0] == '\0')
    {
        // 客户端还没有信任记录：拒绝本次连接并显式告知上层，由上层询问用户是否信任。
        fprintf(stderr, "QTERM-CERT-NEED-TRUST\n");
        fflush(stderr);
        certificate_rejected = TRUE;
        decision = 0;
    }
    else if ((actual[0] != '\0') && (strcmp(expected, actual) == 0))
    {
        decision = 1;
    }
    else
    {
        // 指纹不一致：可能是服务器换证，也可能是中间人，一律拒绝。
        fprintf(stderr, "QTERM-CERT-MISMATCH\n");
        fflush(stderr);
        certificate_rejected = TRUE;
        decision = 0;
    }

    free(subject);
    free(issuer);
    return decision;
}

// ---- 剪贴板互通（FreeRDP cliprdr 虚拟通道）----
//
// 设计取舍：
// * 直接把 FreeRDP cliprdr 客户端的回调挂到我们的 rdpContext 上；helper 不做文件剪贴板，
//   只处理 CF_UNICODETEXT（UTF-16LE）与 CF_TEXT（ASCII 回退），其它格式一律忽略。
// * 远端 → 本机：服务器每次发来格式清单就**主动索取一次文本数据**（不等上层来要），
//   这样"远端复制了什么"能立刻变成一行契约行推给上层；内容没变则不重复推（去重）。
// * 本机 → 远端：事件 7 送来文本后把它当作本机剪贴板内容，向服务器重发一次格式清单；
//   服务器来索取数据时再按 UTF-16LE 应答，远端粘贴拿到的是当时最新的文本。
// * 剪贴板内容可能敏感：helper 只在内存里保存，绝不落盘；日志里只出现长度/格式号/base64 契约行。

// ---- 退出路径（孤儿回收）----
//
// 上层（Qterm）被强杀时，helper 必须自己退出，否则它会一直占着服务器上的 RDP 会话
// （Windows Server 只有 2 个会话，用户可能因此再也连不上）。两条触发条件：
//   1) stdin 读到 EOF —— 父进程关掉了输入管道（正常退出或 kill -9 都会走到这里）；
//   2) stdin 还开着，但 getppid() 变成 1 —— 进程已被 launchd/init 收养，说明父进程没了。
// stdout 只允许帧，所以退出原因用一行 stderr 契约行上报：QTERM-HELPER-EXIT <原因>。
// 原因是下面几种之一：stdin-eof / stdin-error / parent-gone / close-event /
// remote-disconnect / wait-failed。
static const char* helper_exit_reason = NULL;
static volatile LONG helper_exit_reported = 0;

static void helper_request_exit(const char* reason)
{
    // 只记第一个原因：后续（例如断开时又发现 EOF）不覆盖，方便上层定位真实起因。
    if (!helper_exit_reason)
        helper_exit_reason = reason;
}

// 只上报一次：主流程与看门狗线程都可能走到这里（用 WinPR 的原子交换避免打印两行）。
static void helper_report_exit(void)
{
    if (InterlockedCompareExchange(&helper_exit_reported, 1, 0) != 0)
        return;
    fprintf(stderr, "QTERM-HELPER-EXIT %s\n",
            helper_exit_reason ? helper_exit_reason : "remote-disconnect");
    // 退出前把两条输出流都刷干净：stdout 里可能还有半帧没写出去。
    fflush(stderr);
    fflush(stdout);
}

// 父进程还在吗？被 launchd/init 收养后 getppid() 会变成 1。
static BOOL helper_parent_gone(void)
{
    return getppid() == 1;
}

// stdin 是否已经到 EOF。这里只"探测"不读取：读一个字节就会抢走主循环的事件。
// 管道写端全部关闭时 poll 会给出 POLLHUP；只要还有数据可读就让主循环先读完（那时它自己会看到 EOF）。
static BOOL helper_stdin_closed(void)
{
#ifndef _WIN32
    struct pollfd probe = { 0 };
    probe.fd = fileno(stdin);
    probe.events = POLLIN;
    const int rc = poll(&probe, 1, 0);
    if (rc <= 0)
        return FALSE;
    return ((probe.revents & POLLHUP) != 0) && ((probe.revents & POLLIN) == 0);
#else
    return FALSE;
#endif
}

// ---- 兜底看门狗（父进程消失 / stdin 关闭）----
//
// 为什么需要独立线程：主循环只有在 freerdp_connect 返回之后才开始检查 stdin 与 ppid。
// 如果 helper 卡在连接握手里（服务端不回包——实测该状态可持续数十秒）或卡在断开收尾，
// 主线程就永远不会发现父进程已经死了，进程会一直占着服务器上的 RDP 会话：
// 这就是"Qterm 被强杀后 helper 变孤儿、用户再也连不上"的直接成因。
// 看门狗每秒检查一次（Sleep 不空转占 CPU），发现异常先请求中止连接让主流程走干净退出，
// 宽限期内没退就强制 _exit(0)：宁可少一次优雅断开，也不能留孤儿占着会话。
static HANDLE helper_watchdog_start(rdpContext* context);
// 主流程释放上下文之前必须先让看门狗线程停下：它会在别的线程里访问 context。
static volatile LONG helper_watchdog_stop = 0;

static DWORD WINAPI helper_watchdog_main(LPVOID arg)
{
    rdpContext* context = (rdpContext*)arg;
    for (;;)
    {
        // 以 100ms 为片睡满 1 秒：父进程检查仍然"每秒最多一次"，但收尾时能立刻停下线程。
        for (int slice = 0; slice < 10; slice++)
        {
            Sleep(HELPER_WATCHDOG_INTERVAL_MS / 10);
            if (helper_watchdog_stop)
                return 0;
        }
        if (helper_exit_reported != 0)
            return 0; // 主流程已经在收尾，不要再去碰 context

        const char* reason = NULL;
        if (helper_parent_gone())
            reason = "parent-gone";
        else if (helper_stdin_closed())
            reason = "stdin-eof";
        if (!reason)
            continue;

        helper_request_exit(reason);
        // 卡在 freerdp_connect 里的主线程会因为这次中止立刻失败返回，从而走到统一的收尾路径。
        if (context)
            (void)freerdp_abort_connect_context(context);

        for (DWORD waited = 0; waited < HELPER_WATCHDOG_GRACE_MS; waited += 100)
        {
            Sleep(100);
            if (helper_watchdog_stop || (helper_exit_reported != 0))
                return 0; // 主流程已经上报并即将退出，交给它收尾
        }

        // 宽限期到了还没退出：自己收工。进程退出会关闭 RDP 连接，服务端随即回收会话。
        helper_report_exit();
        fflush(NULL);
        _exit(0);
    }
    return 0;
}

static HANDLE helper_watchdog_start(rdpContext* context)
{
    DWORD thread_id = 0;
    HANDLE thread = CreateThread(NULL, 0, helper_watchdog_main, context, 0, &thread_id);
    return thread;
}

// 停止并等待看门狗线程退出。只有确认线程已退出后，调用方才能释放 context；
// 等待失败时宁可直接结束进程，也不能让仍可能访问 context 的线程留下 UAF。
static BOOL helper_watchdog_stop_and_join(HANDLE watchdog)
{
    if (!watchdog)
        return TRUE;
    InterlockedExchange(&helper_watchdog_stop, 1);
    const DWORD status = WaitForSingleObject(watchdog, INFINITE);
    CloseHandle(watchdog);
    return status == WAIT_OBJECT_0;
}

// 读到半个事件/半个负载时必须继续读：把短读当成新事件会让事件流永久错位。
static BOOL read_exact(void* buffer, size_t length)
{
    BYTE* cursor = (BYTE*)buffer;
    size_t remaining = length;
    while (remaining > 0)
    {
        const size_t count = fread(cursor, 1, remaining, stdin);
        if (count == 0)
        {
            if (ferror(stdin))
            {
                fprintf(stderr, "FreeRDP helper 读取 stdin 失败\n");
                helper_request_exit("stdin-error");
                return FALSE;
            }
            if (feof(stdin))
            {
                if (remaining != length)
                    fprintf(stderr, "FreeRDP helper 输入流在半个事件/负载中结束\n");
                helper_request_exit("stdin-eof");
                return FALSE;
            }
            clearerr(stdin);
            continue;
        }
        cursor += count;
        remaining -= count;
    }
    return TRUE;
}

// 把已经声明长度、但不该保留（超限/格式不匹配）的负载读干净再丢弃，保证后续事件仍然对齐。
static BOOL discard_exact(size_t length)
{
    BYTE scratch[4096];
    while (length > 0)
    {
        const size_t chunk = (length < sizeof(scratch)) ? length : sizeof(scratch);
        if (!read_exact(scratch, chunk))
            return FALSE;
        length -= chunk;
    }
    return TRUE;
}

// 远端剪贴板文本上报（契约行）。
// text == NULL 或 length == 0 表示"远端没有文本/被清空"。force 用于事件 8 的手动刷新（跳过去重）。
static void clipboard_report_remote(helperContext* helper, const char* text, size_t length, BOOL force)
{
    const BOOL empty = (!text) || (length == 0);
    if (!force && helper->remote_report_valid && (empty == helper->remote_report_empty))
    {
        if (empty)
            return;
        if ((helper->remote_length == length) && (memcmp(helper->remote_text, text, length) == 0))
            return;
    }

    if (empty)
    {
        fprintf(stderr, "QTERM-CLIPBOARD-EMPTY\n");
    }
    else
    {
        // 文本里可能有换行/控制字符，直接打印会破坏"一行一条"的 stderr 协议，所以用 base64。
        char* encoded = crypto_base64_encode((const BYTE*)text, length);
        if (!encoded)
        {
            fprintf(stderr, "FreeRDP helper 剪贴板文本 base64 编码失败（%zu 字节）\n", length);
            return;
        }
        fprintf(stderr, "QTERM-CLIPBOARD-B64 %s\n", encoded);
        free(encoded);
    }
    fflush(stderr);

    // 记住这次上报的内容用于去重；分配失败只影响去重，不影响已发出的契约行。
    char* copy = NULL;
    if (!empty)
    {
        copy = malloc(length + 1U);
        if (copy)
        {
            memcpy(copy, text, length);
            copy[length] = '\0';
        }
    }
    free(helper->remote_text);
    helper->remote_text = copy;
    helper->remote_length = copy ? length : 0;
    // 极端情况下（记录用的副本分配失败）不记住状态，下一次变化仍会重新上报。
    helper->remote_report_valid = (copy != NULL) || empty;
    helper->remote_report_empty = empty;
}

// 远端 CF_UNICODETEXT 数据是 UTF-16LE；Windows 侧一般带一个结尾 NUL，也可能不带，统一裁掉。
static char* clipboard_decode_unicode(const BYTE* data, UINT32 length, size_t* output_length)
{
    const size_t count = (size_t)length / sizeof(WCHAR);
    if (count == 0)
        return NULL;

    WCHAR* wide = malloc((count + 1U) * sizeof(WCHAR));
    if (!wide)
        return NULL;
    memcpy(wide, data, count * sizeof(WCHAR));
    wide[count] = 0;

    size_t wide_length = count;
    while ((wide_length > 0) && (wide[wide_length - 1] == 0))
        wide_length--;

    size_t utf8_length = 0;
    char* utf8 = ConvertWCharNToUtf8Alloc(wide, wide_length, &utf8_length);
    free(wide);
    if (!utf8)
        return NULL;
    *output_length = utf8_length;
    return utf8;
}

// CF_TEXT 是服务器 ANSI 代码页的 8 位文本。helper 不猜代码页（猜错会静默乱码），
// 按 Latin-1 逐字节展开成 UTF-8：纯 ASCII 与原样一致，非 ASCII 至少不丢字节。
static char* clipboard_decode_text(const BYTE* data, UINT32 length, size_t* output_length)
{
    size_t count = (size_t)length;
    while ((count > 0) && (data[count - 1] == 0))
        count--;
    if (count == 0)
        return NULL;

    char* utf8 = malloc(count * 2U + 1U);
    if (!utf8)
        return NULL;
    size_t written = 0;
    for (size_t index = 0; index < count; index++)
    {
        const unsigned char current = (unsigned char)data[index];
        if (current < 0x80U)
            utf8[written++] = (char)current;
        else
        {
            utf8[written++] = (char)(0xC0U | (current >> 6U));
            utf8[written++] = (char)(0x80U | (current & 0x3FU));
        }
    }
    utf8[written] = '\0';
    *output_length = written;
    return utf8;
}

// 事件 7 设置本机剪贴板文本：只保留一份 UTF-8 副本，不落盘。
static BOOL clipboard_store_local_text(helperContext* helper, const BYTE* data, size_t length)
{
    char* copy = NULL;
    if (length > 0)
    {
        if (!data)
            return FALSE;
        copy = malloc(length + 1U);
        if (!copy)
            return FALSE;
        memcpy(copy, data, length);
        copy[length] = '\0';
    }
    free(helper->local_text);
    helper->local_text = (BYTE*)copy;
    helper->local_length = length;
    helper->local_ready = TRUE;
    // 只打印长度：剪贴板内容可能敏感，明文绝不进日志。
    fprintf(stderr, "FreeRDP helper 本机剪贴板文本已更新（%zu 字节）\n", length);
    fflush(stderr);
    return TRUE;
}

static BOOL clipboard_local_text_is_ascii(const helperContext* helper)
{
    for (size_t index = 0; index < helper->local_length; index++)
    {
        if (helper->local_text[index] >= 0x80U)
            return FALSE;
    }
    return TRUE;
}

// 把本机剪贴板清单发给服务器：服务器据此知道"客户端拥有剪贴板"，
// 之后用户在远端粘贴时才会向我们索取数据。
static UINT clipboard_send_local_format_list(helperContext* helper)
{
    CliprdrClientContext* cliprdr = helper->cliprdr;
    if (!cliprdr || !cliprdr->ClientFormatList)
        return CHANNEL_RC_OK;

    CLIPRDR_FORMAT formats[2] = { { 0 } };
    UINT32 count = 0;
    if (helper->local_ready && (helper->local_length > 0))
    {
        // CF_UNICODETEXT 一定能声明；只有纯 ASCII 文本才额外声明 CF_TEXT，
        // 否则旧应用可能选中 ANSI 回退，把中文变成问号。
        formats[count].formatId = CLIPRDR_FORMAT_CF_UNICODETEXT;
        formats[count].formatName = NULL;
        count++;
        if (clipboard_local_text_is_ascii(helper))
        {
            formats[count].formatId = CLIPRDR_FORMAT_CF_TEXT;
            formats[count].formatName = NULL;
            count++;
        }
    }

    CLIPRDR_FORMAT_LIST format_list = { 0 };
    format_list.common.msgType = CB_FORMAT_LIST;
    format_list.numFormats = count;
    format_list.formats = (count > 0) ? formats : NULL;
    // 空清单合法：它表示"本机剪贴板当前没有文本"。
    return cliprdr->ClientFormatList(cliprdr, &format_list);
}

static BOOL clipboard_format_is_unicode(const CLIPRDR_FORMAT* format)
{
    if (format->formatId == CLIPRDR_FORMAT_CF_UNICODETEXT)
        return TRUE;
    return format->formatName &&
           ((strcmp(format->formatName, "CF_UNICODETEXT") == 0) ||
            (strcmp(format->formatName, "UnicodeText") == 0));
}

static BOOL clipboard_format_is_text(const CLIPRDR_FORMAT* format)
{
    if (format->formatId == CLIPRDR_FORMAT_CF_TEXT)
        return TRUE;
    return format->formatName && (strcmp(format->formatName, "CF_TEXT") == 0);
}

static UINT clipboard_request_remote_text(helperContext* helper, CliprdrClientContext* cliprdr,
                                          UINT32 format_id)
{
    CLIPRDR_FORMAT_DATA_REQUEST request = { 0 };
    request.common.msgType = CB_FORMAT_DATA_REQUEST;
    request.requestedFormatId = format_id;
    helper->remote_request_format = format_id;
    helper->remote_request_pending = TRUE;
    if (!cliprdr->ClientFormatDataRequest)
        return CHANNEL_RC_OK;
    return cliprdr->ClientFormatDataRequest(cliprdr, &request);
}

// 服务器 Monitor Ready：先回能力集，再把本机剪贴板清单发过去。
static UINT helper_cliprdr_monitor_ready(CliprdrClientContext* context,
                                         const CLIPRDR_MONITOR_READY* monitor_ready)
{
    (void)monitor_ready;
    helperContext* helper = (context && context->custom) ? (helperContext*)context->custom : NULL;
    if (!context || !helper)
        return CHANNEL_RC_OK;

    // 声明支持长格式名；helper 不做文件剪贴板，所以不申请 CB_STREAM_FILECLIP_ENABLED 等能力位。
    CLIPRDR_GENERAL_CAPABILITY_SET general = { 0 };
    general.capabilitySetType = CB_CAPSTYPE_GENERAL;
    general.capabilitySetLength = CB_CAPSTYPE_GENERAL_LEN;
    general.version = CB_CAPS_VERSION_2;
    general.generalFlags = CB_USE_LONG_FORMAT_NAMES;

    CLIPRDR_CAPABILITIES capabilities = { 0 };
    capabilities.common.msgType = CB_CLIP_CAPS;
    capabilities.cCapabilitiesSets = 1;
    capabilities.capabilitySets = (CLIPRDR_CAPABILITY_SET*)&general;

    if (context->ClientCapabilities)
    {
        const UINT status = context->ClientCapabilities(context, &capabilities);
        if (status != CHANNEL_RC_OK)
            return status;
    }
    return clipboard_send_local_format_list(helper);
}

// 服务器发来剪贴板格式清单（远端剪贴板内容变了，或者被清空）。
static UINT helper_cliprdr_server_format_list(CliprdrClientContext* context,
                                              const CLIPRDR_FORMAT_LIST* format_list)
{
    helperContext* helper = (context && context->custom) ? (helperContext*)context->custom : NULL;
    if (!context || !helper || !format_list)
        return CHANNEL_RC_OK;

    // 先回 CB_RESPONSE_OK：MS-RDPECLIP 要求客户端确认收到清单，不回执服务器可能不再发数据。
    if (context->ClientFormatListResponse)
    {
        CLIPRDR_FORMAT_LIST_RESPONSE response = { 0 };
        response.common.msgType = CB_FORMAT_LIST_RESPONSE;
        response.common.msgFlags = CB_RESPONSE_OK;
        context->ClientFormatListResponse(context, &response);
    }

    UINT32 unicode_id = 0;
    UINT32 text_id = 0;
    for (UINT32 index = 0; index < format_list->numFormats; index++)
    {
        const CLIPRDR_FORMAT* format = &format_list->formats[index];
        if ((unicode_id == 0) && clipboard_format_is_unicode(format))
            unicode_id = format->formatId;
        else if ((text_id == 0) && clipboard_format_is_text(format))
            text_id = format->formatId;
    }

    const UINT32 chosen = (unicode_id != 0) ? unicode_id : text_id;
    if (chosen == 0)
    {
        // 清单里没有文本格式（只复制了图片/文件，或远端剪贴板被清空）→ 上报"没有文本"（去重）。
        helper->remote_request_format = 0;
        helper->remote_request_pending = FALSE;
        clipboard_report_remote(helper, NULL, 0, FALSE);
        return CHANNEL_RC_OK;
    }

    // 每次清单都重新索取：远端文本换了内容但格式号还是 13，只按清单去重会漏掉新内容。
    return clipboard_request_remote_text(helper, context, chosen);
}

// 服务器来索取本机剪贴板数据（用户在远端按了粘贴）。
static UINT helper_cliprdr_server_format_data_request(
    CliprdrClientContext* context, const CLIPRDR_FORMAT_DATA_REQUEST* format_data_request)
{
    helperContext* helper = (context && context->custom) ? (helperContext*)context->custom : NULL;
    if (!context || !helper || !format_data_request)
        return CHANNEL_RC_OK;

    BYTE* buffer = NULL;
    UINT32 length = 0;
    if (helper->local_ready && (helper->local_length > 0) && helper->local_text)
    {
        if (format_data_request->requestedFormatId == CLIPRDR_FORMAT_CF_UNICODETEXT)
        {
            size_t wide_length = 0;
            WCHAR* wide = ConvertUtf8ToWCharAlloc((const char*)helper->local_text, &wide_length);
            if (wide)
            {
                const size_t bytes = (wide_length + 1U) * sizeof(WCHAR);
                buffer = calloc(bytes, 1U); // calloc 已经把结尾的 UTF-16 NUL 留好了
                if (buffer)
                {
                    memcpy(buffer, wide, wide_length * sizeof(WCHAR));
                    length = (UINT32)bytes;
                }
                free(wide);
            }
        }
        else if (format_data_request->requestedFormatId == CLIPRDR_FORMAT_CF_TEXT)
        {
            // 只有纯 ASCII 文本才会被声明成 CF_TEXT，所以这里可以整块拷贝。
            buffer = calloc(helper->local_length + 1U, 1U);
            if (buffer)
            {
                memcpy(buffer, helper->local_text, helper->local_length);
                length = (UINT32)helper->local_length + 1U;
            }
        }
    }

    CLIPRDR_FORMAT_DATA_RESPONSE response = { 0 };
    response.common.msgType = CB_FORMAT_DATA_RESPONSE;
    // 本机剪贴板为空或格式不匹配时明确回 CB_RESPONSE_FAIL：回空串会把对方的剪贴板清掉。
    response.common.msgFlags = buffer ? CB_RESPONSE_OK : CB_RESPONSE_FAIL;
    response.common.dataLen = buffer ? length : 0;
    response.requestedFormatData = buffer;

    // 只记格式号与长度，不记内容（剪贴板可能含敏感信息）。
    fprintf(stderr, "FreeRDP helper 回应远端粘贴请求（格式 %u，%u 字节）\n",
            format_data_request->requestedFormatId, response.common.dataLen);
    fflush(stderr);

    UINT status = CHANNEL_RC_OK;
    if (context->ClientFormatDataResponse)
        status = context->ClientFormatDataResponse(context, &response);
    free(buffer);
    return status;
}

// 服务器回给我们之前索取的远端剪贴板数据。
static UINT helper_cliprdr_server_format_data_response(
    CliprdrClientContext* context, const CLIPRDR_FORMAT_DATA_RESPONSE* format_data_response)
{
    helperContext* helper = (context && context->custom) ? (helperContext*)context->custom : NULL;
    if (!context || !helper || !format_data_response)
        return CHANNEL_RC_OK;

    helper->remote_request_pending = FALSE;

    const BOOL failed = (format_data_response->common.msgFlags & CB_RESPONSE_FAIL) != 0;
    const BYTE* data = format_data_response->requestedFormatData;
    if (failed || !data || (format_data_response->common.dataLen == 0))
    {
        clipboard_report_remote(helper, NULL, 0, FALSE);
        return CHANNEL_RC_OK;
    }

    size_t text_length = 0;
    char* text = NULL;
    if (helper->remote_request_format == CLIPRDR_FORMAT_CF_UNICODETEXT)
        text = clipboard_decode_unicode(data, format_data_response->common.dataLen, &text_length);
    else
        text = clipboard_decode_text(data, format_data_response->common.dataLen, &text_length);

    if (!text)
    {
        // 转换失败/内容为空：按"没有文本"上报（去重会挡掉重复），不打印明文。
        clipboard_report_remote(helper, NULL, 0, FALSE);
        return CHANNEL_RC_OK;
    }

    clipboard_report_remote(helper, text, text_length, FALSE);
    free(text);
    return CHANNEL_RC_OK;
}

// ---- 动态分辨率（disp / Display Control 虚拟通道）----
//
// 为什么走 disp 而不是重连：RDP 8 起服务端支持 MS-RDPEDISP。客户端在
// "Microsoft::Windows::RDS::DisplayControl" 动态虚拟通道上发一条 monitor layout 请求，
// 服务端接受后走 Deactivate-All / Reactivate-All 重新协商桌面尺寸并重推整屏——
// 会话、进程、连接都保持不变（重连会踢掉当前会话、丢掉正在跑的程序，用户明确不要）。
//
// 为什么必须等 DisplayControlCaps：那条 PDU 才是"服务端真的实现了该通道"的证据
// （FreeRDP 自己的 X11/Windows 客户端也用同一个 activated 门闩）。没有它就发 monitor layout
// 属于未协商行为，可能被静默丢弃——那样反而会把"服务端不支持"误判成"支持但没反应"。

// 契约行（语义冻结）：服务端不支持/忽略了动态分辨率。每次尺寸请求最多一行，避免刷屏。
// stdout 只能是帧，所以走 stderr；调用方不要再往这一行里追加任何字段。
static void helper_report_resize_unsupported(void)
{
    fprintf(stderr, "QTERM-RESIZE-UNSUPPORTED\n");
    fflush(stderr);
}

// 动态分辨率是否启用：**默认启用**（未设置也算启用），只有显式 "0" 关闭。
static BOOL helper_dynamic_resolution_enabled(void)
{
    const char* value = getenv(DYNAMIC_RESOLUTION_ENV);
    return (value == NULL) || (strcmp(value, "0") != 0);
}

// 解析剪贴板设置，返回要设置的 ClipboardFeatureMask（0 = 关闭）。
//
// 取值为空/未知时回落默认（双向 / 仅文本），与上层 NormalizeClipboardSettings 同一口径：
// 上游已经把非法值收敛过一遍，这里再兜一次是为了让"直接手动跑 helper"也不会出现
// 开关含义不明的情况。
static UINT32 helper_clipboard_feature_mask(const char** mode_label, const char** content_label)
{
    const char* mode = getenv(CLIPBOARD_MODE_ENV);
    const char* content = getenv(CLIPBOARD_CONTENT_ENV);
    if (mode == NULL)
    {
        mode = "both";
    }
    if (content == NULL)
    {
        content = "text";
    }

    UINT32 mask = 0;
    if (strcmp(mode, "off") == 0)
    {
        *mode_label = "off";
        // 关闭时内容标签没有意义，统一显示 text（避免诊断行出现"off + 文件"这种矛盾说法）。
        *content_label = "text";
        return 0;
    }
    if (strcmp(mode, "local-to-remote") == 0)
    {
        mask = CLIPBOARD_FLAG_LOCAL_TO_REMOTE;
        *mode_label = "local-to-remote";
    }
    else if (strcmp(mode, "remote-to-local") == 0)
    {
        mask = CLIPBOARD_FLAG_REMOTE_TO_LOCAL;
        *mode_label = "remote-to-local";
    }
    else
    {
        mask = CLIPBOARD_FLAG_LOCAL_TO_REMOTE | CLIPBOARD_FLAG_REMOTE_TO_LOCAL;
        *mode_label = "both";
    }

#if CLIPBOARD_FILE_TRANSFER_IMPLEMENTED
    if (strcmp(content, "text-and-files") == 0)
    {
        mask |= CLIPBOARD_FLAG_LOCAL_TO_REMOTE_FILES | CLIPBOARD_FLAG_REMOTE_TO_LOCAL_FILES;
        *content_label = "text-and-files";
        return mask;
    }
#endif
    *content_label = "text";
    return mask;
}

// 自我降级契约行：动态分辨率开着却一帧都没有。语义冻结（整行只有这个 token）。
// 打完这一行就走**干净退出**（freerdp_disconnect + 释放上下文），退出码 6；绝不用 _exit(0)，
// 因为这不是孤儿回收，而是"能正常断开、但画面不可用"，要让服务端收到正常的断开 PDU。
static void helper_report_fallback_no_frames(void)
{
    fprintf(stderr, "QTERM-RDP-FALLBACK no-frames\n");
    fflush(stderr);
}

// 主循环每轮调用：判定"开了动态分辨率却始终没有帧"，返回 TRUE 表示要干净退出（退出码 6）。
static BOOL helper_check_fallback_no_frames(rdpContext* context)
{
    helperContext* helper = (helperContext*)context;
    if (!helper->fallback_armed || (helper->frames_output > 0))
    {
        // 已经有帧（绝大多数目标）或本来就没武装：正常继续，不再做任何判定。
        helper->fallback_armed = FALSE;
        return FALSE;
    }
    if (GetTickCount64() < helper->fallback_deadline)
        return FALSE;

    helper->fallback_armed = FALSE;
    helper->fallback_fired = TRUE;
    fprintf(stderr,
            "FreeRDP helper 动态分辨率启用后 %u 秒内没有任何帧，判定该目标不适用，"
            "干净退出（退出码 %d）\n",
            (unsigned)(DYNAMIC_RESOLUTION_FALLBACK_MS / 1000U),
            EXIT_DYNAMIC_RESOLUTION_FALLBACK);
    helper_report_fallback_no_frames();
    return TRUE;
}

// 服务端的 DisplayControlCaps PDU：能力协商完成，此时才允许发 monitor layout。
static UINT helper_disp_control_caps(DispClientContext* context, UINT32 max_monitors,
                                     UINT32 max_area_factor_a, UINT32 max_area_factor_b)
{
    (void)max_area_factor_a;
    (void)max_area_factor_b;
    helperContext* helper = (context && context->custom) ? (helperContext*)context->custom : NULL;
    if (!helper)
        return CHANNEL_RC_OK;
    helper->disp_activated = TRUE;
    fprintf(stderr, "FreeRDP helper 动态分辨率能力已协商（最多 %u 个显示器）\n", max_monitors);
    fflush(stderr);
    return CHANNEL_RC_OK;
}

// 事件 9 的落地：请求服务端把桌面改成 width×height。
// 返回值只表示"事件已经被完整消费、事件流没有坏"（FALSE 会让主循环退出），
// **不**代表请求成功：服务端不支持时也必须继续出帧，绝不能因此结束会话。
static BOOL helper_request_resize(rdpContext* context, UINT32 width, UINT32 height)
{
    if (!context)
        return TRUE;
    helperContext* helper = (helperContext*)context;
    rdpGdi* gdi = context->gdi;

    // 未启用（QTERM_RDP_DYNAMIC_RESOLUTION=0）：按契约只回一行"不支持"，**绝不**向服务端发任何东西，
    // 也绝不碰连接状态——该能力在现代目标上出现过"连上但 0 帧"的真机回归（见常量处说明）。
    if (!helper->dynamic_resolution_enabled)
    {
        fprintf(stderr, "FreeRDP helper 动态分辨率已禁用（%s=0），本次尺寸请求不发送\n",
                DYNAMIC_RESOLUTION_ENV);
        helper_report_resize_unsupported();
        return TRUE;
    }

    // 与当前桌面同尺寸：没有可请求的变化。服务端即使"接受"也不会触发重新协商，
    // 不能因此把它误判成不支持（否则超时兜底会打出一条假契约行）。
    if (gdi && ((UINT32)gdi->width == width) && ((UINT32)gdi->height == height))
    {
        fprintf(stderr, "FreeRDP helper 动态分辨率请求与当前桌面同尺寸（%ux%u），已忽略\n", width,
                height);
        fflush(stderr);
        return TRUE;
    }

    DISPLAY_CONTROL_MONITOR_LAYOUT layout = { 0 };
    rdpSettings* settings = context->settings;
    layout.Flags = DISPLAY_CONTROL_MONITOR_PRIMARY;
    layout.Left = 0;
    layout.Top = 0;
    layout.Width = width;
    layout.Height = height;
    // 物理尺寸只影响服务端推断 DPI：按 75 DPI 换算成毫米（与 FreeRDP X11 客户端同口径）。
    // 用整数四舍五入，避免为一个纯装饰字段引入浮点/数学库依赖。
    layout.PhysicalWidth = (width * 254U + 375U) / 750U;
    layout.PhysicalHeight = (height * 254U + 375U) / 750U;
    if (settings)
    {
        layout.Orientation = freerdp_settings_get_uint16(settings, FreeRDP_DesktopOrientation);
        layout.DesktopScaleFactor =
            freerdp_settings_get_uint32(settings, FreeRDP_DesktopScaleFactor);
        layout.DeviceScaleFactor =
            freerdp_settings_get_uint32(settings, FreeRDP_DeviceScaleFactor);
    }

    // 没有 disp 通道 = 服务端根本没有这个能力（例如 Windows Server 2008 R2）；
    // 通道在但能力 PDU 还没来 = 协商尚未完成，同样先如实上报，不冒着发未协商 PDU 的风险。
    if (!helper->disp || !helper->disp_activated)
    {
        fprintf(stderr, "FreeRDP helper 动态分辨率不可用：%s\n",
                helper->disp ? "disp 通道尚未完成能力协商" : "本次会话没有 disp 通道（服务端不支持）");
        helper_report_resize_unsupported();
        // 之前的待确认请求也一并了结，避免同一件事再被超时兜底上报一次。
        helper->resize_pending = FALSE;
        return TRUE;
    }

    const UINT status = helper->disp->SendMonitorLayout
                            ? helper->disp->SendMonitorLayout(helper->disp, 1, &layout)
                            : CHANNEL_RC_NOT_INITIALIZED;
    if (status != CHANNEL_RC_OK)
    {
        fprintf(stderr, "FreeRDP helper disp 通道拒绝了本次尺寸请求（状态 %u）\n", status);
        helper_report_resize_unsupported();
        helper->resize_pending = FALSE;
        return TRUE;
    }

    // 请求已发出：服务端接受后会通过 Reactivate-All 改尺寸并触发 DesktopResize 回调，
    // 那时撤销这里的兜底；一直没回应则由主循环判超时（helper_check_resize_timeout）。
    helper->resize_pending = TRUE;
    helper->resize_pending_width = width;
    helper->resize_pending_height = height;
    helper->resize_pending_deadline = GetTickCount64() + RESIZE_ACK_TIMEOUT_MS;
    fprintf(stderr, "FreeRDP helper 已请求桌面尺寸 %ux%u（等待服务端重新协商）\n", width, height);
    fflush(stderr);
    return TRUE;
}

// 主循环每秒至少调用一次：把"请求已发出、服务端却始终没有重新协商尺寸"判为不支持。
// 只上报一次并清除 pending，不重连、不阻塞出帧——桌面保持旧尺寸，画面继续按旧尺寸出。
static void helper_check_resize_timeout(rdpContext* context)
{
    helperContext* helper = (helperContext*)context;
    if (!helper->resize_pending || (GetTickCount64() < helper->resize_pending_deadline))
        return;
    helper->resize_pending = FALSE;
    fprintf(stderr,
            "FreeRDP helper 动态分辨率请求超时未生效（%u 秒内服务端没有重新协商尺寸，"
            "请求为 %ux%u）\n",
            (unsigned)(RESIZE_ACK_TIMEOUT_MS / 1000U), helper->resize_pending_width,
            helper->resize_pending_height);
    helper_report_resize_unsupported();
}

// 通道连上：拿到 CliprdrClientContext / DispClientContext，挂上我们关心的回调。
static void helper_on_channel_connected(void* context, const ChannelConnectedEventArgs* e)
{
    if (!context || !e || !e->name)
        return;

    helperContext* helper = (helperContext*)context;

    if (strcmp(e->name, DISP_DVC_CHANNEL_NAME) == 0)
    {
        // 未启用时连 custom / 回调都不挂：disp 通道本身可能仍被 FreeRDP 按默认设置加载
        // （settings 里 SupportDisplayControl 默认就是 TRUE），但我们必须让它的行为与
        // 引入本功能之前完全一致——默认路径不参与任何 monitor layout 协商。
        if (!helper->dynamic_resolution_enabled)
            return;

        DispClientContext* disp = (DispClientContext*)e->pInterface;
        if (!disp)
            return;
        // 回调里只能从通道上下文反查 helper，所以把 helper 塞进通道自带的 custom 字段。
        disp->custom = helper;
        disp->DisplayControlCaps = helper_disp_control_caps;
        helper->disp = disp;
        helper->disp_activated = FALSE;
        fprintf(stderr, "FreeRDP helper 动态分辨率通道已就绪（等待能力协商）\n");
        fflush(stderr);
        return;
    }

    if (strcmp(e->name, CLIPRDR_SVC_CHANNEL_NAME) != 0)
        return;

    // 关闭剪贴板（方向=off）时不挂任何回调：即使通道因为别的设置仍然被创建，
    // 我们也不会把本机内容发布出去、也不会去索取远端内容。这是 pre_connect 里
    // RedirectClipboard=FALSE 之外的第二道闸。
    if (helper->clipboard_feature_mask == 0)
        return;

    CliprdrClientContext* cliprdr = (CliprdrClientContext*)e->pInterface;
    if (!cliprdr)
        return;

    // 回调里只能从 context 反查 helper，所以把上下文塞进通道自带的 custom 字段。
    cliprdr->custom = helper;
    cliprdr->MonitorReady = helper_cliprdr_monitor_ready;
    cliprdr->ServerFormatList = helper_cliprdr_server_format_list;
    cliprdr->ServerFormatDataRequest = helper_cliprdr_server_format_data_request;
    cliprdr->ServerFormatDataResponse = helper_cliprdr_server_format_data_response;
    helper->cliprdr = cliprdr;

    fprintf(stderr, "FreeRDP helper 剪贴板通道已就绪\n");
    fflush(stderr);
}

static void helper_on_channel_disconnected(void* context, const ChannelDisconnectedEventArgs* e)
{
    if (!context || !e || !e->name)
        return;

    helperContext* helper = (helperContext*)context;

    if (strcmp(e->name, DISP_DVC_CHANNEL_NAME) == 0)
    {
        helper->disp = NULL;
        helper->disp_activated = FALSE;
        helper->resize_pending = FALSE;
        return;
    }

    if (strcmp(e->name, CLIPRDR_SVC_CHANNEL_NAME) != 0)
        return;

    helper->cliprdr = NULL;
    helper->remote_request_format = 0;
    helper->remote_request_pending = FALSE;
}

// 事件 8：把当前远端剪贴板状态立刻再上报一次（界面手动刷新），跳过去重。
// 还没有任何远端状态时，如果知道可请求的格式就重新索取一次，否则如实上报"没有文本"。
static void clipboard_force_resend(helperContext* helper)
{
    if (helper->remote_report_valid)
    {
        clipboard_report_remote(helper,
                                helper->remote_report_empty ? NULL : helper->remote_text,
                                helper->remote_report_empty ? 0 : helper->remote_length, TRUE);
        return;
    }

    if (helper->cliprdr && (helper->remote_request_format != 0) &&
        !helper->remote_request_pending)
    {
        clipboard_request_remote_text(helper, helper->cliprdr, helper->remote_request_format);
        return;
    }

    clipboard_report_remote(helper, NULL, 0, TRUE);
}

static void clipboard_release(helperContext* helper)
{
    free(helper->local_text);
    helper->local_text = NULL;
    helper->local_length = 0;
    helper->local_ready = FALSE;

    free(helper->remote_text);
    helper->remote_text = NULL;
    helper->remote_length = 0;
    helper->remote_report_valid = FALSE;
    helper->remote_report_empty = FALSE;

    helper->cliprdr = NULL;
    helper->remote_request_format = 0;
    helper->remote_request_pending = FALSE;
}

static BOOL helper_begin_paint(rdpContext* context)
{
    if (!context || !context->gdi || !context->gdi->primary || !context->gdi->primary->hdc ||
        !context->gdi->primary->hdc->hwnd || !context->gdi->primary->hdc->hwnd->invalid)
        return FALSE;
    context->gdi->primary->hdc->hwnd->invalid->null = TRUE;
    return TRUE;
}

static BOOL helper_end_paint(rdpContext* context)
{
    if (!context || !context->gdi || !context->gdi->primary || !context->gdi->primary->hdc ||
        !context->gdi->primary->hdc->hwnd)
        return FALSE;

    rdpGdi* gdi = context->gdi;
    HGDI_WND window = gdi->primary->hdc->hwnd;
    if (!window->invalid || window->invalid->null)
        return TRUE;

    const INT32 x = window->invalid->x;
    const INT32 y = window->invalid->y;
    const INT32 width = window->invalid->w;
    const INT32 height = window->invalid->h;
    if (x < 0 || y < 0 || width <= 0 || height <= 0 || x + width > gdi->width ||
        y + height > gdi->height)
        return FALSE;

    const uint32_t stride = (uint32_t)width * 4U;
    const uint32_t payload_length = stride * (uint32_t)height;
    BYTE header[FRAME_HEADER_BYTES] = { 0 };
    helperContext* helper = (helperContext*)context;
    write_u64_le(header, helper->sequence++);
    write_u32_le(header + 8, (uint32_t)gdi->width);
    write_u32_le(header + 12, (uint32_t)gdi->height);
    write_u32_le(header + 16, (uint32_t)x);
    write_u32_le(header + 20, (uint32_t)y);
    write_u32_le(header + 24, (uint32_t)width);
    write_u32_le(header + 28, (uint32_t)height);
    write_u32_le(header + 32, stride);
    write_u32_le(header + 36, 2U); // Qterm IPC 的 RGBA8888。
    write_u32_le(header + 40, payload_length);

    if (!write_all(header, sizeof(header)))
        goto fail;
    for (INT32 row = 0; row < height; row++)
    {
        const BYTE* source = gdi->primary_buffer + (size_t)(y + row) * gdi->stride +
                             (size_t)x * 4U;
        if (!write_all(source, stride))
            goto fail;
    }
    if (fflush(stdout) != 0)
        goto fail;

    // 到这说明一帧完整写出去了：动态分辨率"没有帧"的自我降级判定只认这个计数。
    helper->frames_output++;
    window->invalid->null = TRUE;
    window->ninvalid = 0;
    return TRUE;

fail:
    helper->output_failed = TRUE;
    return FALSE;
}

static BOOL helper_desktop_resize(rdpContext* context)
{
    if (!context || !context->gdi || !context->settings)
        return FALSE;

    const UINT32 width = freerdp_settings_get_uint32(context->settings, FreeRDP_DesktopWidth);
    const UINT32 height = freerdp_settings_get_uint32(context->settings, FreeRDP_DesktopHeight);
    // 服务端已经走完 Reactivate-All 重新协商：撤销尺寸请求的超时兜底。
    // gdi_resize 会把 gdi->width/height 换成新值，而 helper_end_paint 的 44 字节帧头
    // （偏移 8/12）直接读 gdi->width/height，所以**后续帧头自然是新尺寸**，无需额外同步。
    helperContext* helper = (helperContext*)context;
    helper->resize_pending = FALSE;
    fprintf(stderr, "FreeRDP helper 桌面尺寸已重新协商为 %ux%u\n", width, height);
    fflush(stderr);
    return gdi_resize(context->gdi, width, height);
}

static BOOL helper_pre_connect(freerdp* instance)
{
    if (!instance || !instance->context || !instance->context->settings)
        return FALSE;

    // 由应用（本 helper）独自负责证书校验：这样 FreeRDP 不会走 known_hosts / 系统 CA 的
    // 自动接受路径（否则「证书已存过」或「公信 CA 签发」时会跳过指纹比对而静默放行），
    // 每一次连接都必须经过 helper_verify_x509_certificate。
    if (!freerdp_settings_set_bool(instance->context->settings,
                                   FreeRDP_ExternalCertificateManagement, TRUE))
        return FALSE;

    // 剪贴板通道：由主机的剪贴板方向决定（QTERM_RDP_CLIPBOARD_MODE，见常量处的说明）。
    //
    //   mask != 0 → 打开通道并设置方向位。FreeRDP 的客户端会按这些位过滤"本机格式列表"
    //               与"远端格式列表/文件请求"，因此关闭某个方向是**通道级**保证。
    //   mask == 0 → 显式关掉通道（RedirectClipboard=FALSE）。FreeRDP 在客户端模式下默认打开，
    //               不显式关的话"关闭剪贴板"只关掉了我们自己的回调，两侧仍会互发内容。
    const helperContext* clipboard_helper = (const helperContext*)instance->context;
    if (clipboard_helper->clipboard_feature_mask == 0)
    {
        if (!freerdp_settings_set_bool(instance->context->settings, FreeRDP_RedirectClipboard, FALSE))
            return FALSE;
    }
    else
    {
        if (!freerdp_settings_set_bool(instance->context->settings, FreeRDP_RedirectClipboard, TRUE))
            return FALSE;
        if (!freerdp_settings_set_uint32(instance->context->settings, FreeRDP_ClipboardFeatureMask,
                                         clipboard_helper->clipboard_feature_mask))
            return FALSE;
    }

    // Unicode 键盘（事件 6）：FreeRDP 的命令行解析里有一句
    // "Disable unicode input unless requested"，会把 UnicodeInput 强制置回 FALSE；
    // 不在这里重新打开的话，freerdp_input_send_unicode_keyboard_event 只会打一行
    // "Unicode input not supported by server." 然后把事件丢掉（中文等非 ASCII 输入全靠它）。
    if (!freerdp_settings_set_bool(instance->context->settings, FreeRDP_UnicodeInput, TRUE))
        return FALSE;

    // 动态分辨率（事件 9）：**默认关闭**，只有 QTERM_RDP_DYNAMIC_RESOLUTION=1 时才打开 disp 通道与
    // 动态分辨率（等价命令行 +dynamic-resolution）。未启用时这里一行都不做，
    // 让 SupportDisplayControl / DynamicResolutionUpdate 保持 FreeRDP 的原值，默认路径与改动前一致
    // （真机回归说明见常量定义处）。DynamicResolutionUpdate 只是客户端侧的开关，
    // 一并置位是为了与 +dynamic-resolution 的行为完全一致。
    if (((helperContext*)instance->context)->dynamic_resolution_enabled)
    {
        if (!freerdp_settings_set_bool(instance->context->settings, FreeRDP_SupportDisplayControl,
                                       TRUE))
            return FALSE;
        if (!freerdp_settings_set_bool(instance->context->settings,
                                       FreeRDP_DynamicResolutionUpdate, TRUE))
            return FALSE;
    }
    // 虚拟通道是 pre_connect 之后才加载并触发 ChannelConnected 的，所以订阅必须放在这里。
    if (PubSub_SubscribeChannelConnected(instance->context->pubSub,
                                         helper_on_channel_connected) < 0)
        return FALSE;
    if (PubSub_SubscribeChannelDisconnected(instance->context->pubSub,
                                            helper_on_channel_disconnected) < 0)
        return FALSE;
    return TRUE;
}

static BOOL helper_post_connect(freerdp* instance)
{
    if (!instance || !instance->context || !instance->context->update)
        return FALSE;
    if (!gdi_init(instance, PIXEL_FORMAT_RGBA32))
        return FALSE;
    instance->context->update->BeginPaint = helper_begin_paint;
    instance->context->update->EndPaint = helper_end_paint;
    instance->context->update->DesktopResize = helper_desktop_resize;

    // 动态分辨率开启时武装"没有帧就降级"的计时：起点是这里（连接完成），窗口 DYNAMIC_RESOLUTION_FALLBACK_MS。
    // 关闭时**不武装**——否则会把"服务器本身就不出图"误判成我们的问题。
    helperContext* helper = (helperContext*)instance->context;
    if (helper->dynamic_resolution_enabled)
    {
        helper->fallback_armed = TRUE;
        helper->fallback_deadline = GetTickCount64() + DYNAMIC_RESOLUTION_FALLBACK_MS;
    }
    return TRUE;
}

static void helper_post_disconnect(freerdp* instance)
{
    if (instance)
    {
        if (instance->context)
            clipboard_release((helperContext*)instance->context);
        gdi_free(instance);
    }
}

static BOOL helper_client_new(freerdp* instance, rdpContext* context)
{
    if (!instance || !context)
        return FALSE;
    instance->PreConnect = helper_pre_connect;
    instance->PostConnect = helper_post_connect;
    instance->PostDisconnect = helper_post_disconnect;
    // 证书指纹固定：挂上 FreeRDP 3 的 X509 校验回调（配合上面的 ExternalCertificateManagement）。
    instance->VerifyX509Certificate = helper_verify_x509_certificate;
    return TRUE;
}

static BOOL process_input(rdpContext* context)
{
    BYTE event[INPUT_BYTES];
    // 先只读 1 字节：这样才能区分「对端在事件边界正常关闭」与「事件被截断」（半个事件）。
    const size_t first = fread(event, 1, 1, stdin);
    if (first == 0)
    {
        if (feof(stdin))
        {
            // 父进程关掉输入管道（正常退出或被强杀）：事件边界上的 EOF 就是"该收工了"。
            helper_request_exit("stdin-eof");
            return FALSE;
        }
        fprintf(stderr, "FreeRDP helper 读取输入事件失败\n");
        helper_request_exit("stdin-error");
        return FALSE;
    }
    if (!read_exact(event + 1, sizeof(event) - 1))
        return FALSE;

    const uint32_t type = read_u32_le(event);
    const UINT16 x = (UINT16)read_u32_le(event + 4);
    const UINT16 y = (UINT16)read_u32_le(event + 8);
    // 事件 9 用满 4 字节当尺寸（虽然当前上限 7680x4320 放得进 16 位，也不做截断，
    // 免得将来放宽范围时这里静默出错）。
    const uint32_t raw_x = read_u32_le(event + 4);
    const uint32_t raw_y = read_u32_le(event + 8);
    const int32_t value = (int32_t)read_u32_le(event + 12);
    const uint32_t flags = read_u32_le(event + 16);
    rdpInput* input = context->input;

    switch (type)
    {
        case INPUT_MOUSE_MOVE:
            return freerdp_input_send_mouse_event(input, PTR_FLAGS_MOVE, x, y);
        case INPUT_MOUSE_BUTTON:
        {
            UINT16 button = 0;
            if (value == 0)
                button = PTR_FLAGS_BUTTON1;
            else if (value == 1)
                button = PTR_FLAGS_BUTTON2;
            else if (value == 2)
                button = PTR_FLAGS_BUTTON3;
            else
                return TRUE;
            if (!freerdp_input_send_mouse_event(input, PTR_FLAGS_MOVE, x, y))
                return FALSE;
            return freerdp_input_send_mouse_event(
                input, button | ((flags & 1U) ? PTR_FLAGS_DOWN : 0), x, y);
        }
        case INPUT_WHEEL:
        {
            int32_t units = value * 120;
            UINT16 wheel = (flags & 2U) ? PTR_FLAGS_HWHEEL : PTR_FLAGS_WHEEL;
            if (units < 0)
            {
                wheel |= PTR_FLAGS_WHEEL_NEGATIVE;
                units = -units;
            }
            wheel |= (UINT16)units & WheelRotationMask;
            return freerdp_input_send_mouse_event(input, wheel, x, y);
        }
        case INPUT_KEY:
        {
            UINT16 keyboard_flags = (flags & 1U) ? KBD_FLAGS_DOWN : KBD_FLAGS_RELEASE;
            if (flags & 2U)
                keyboard_flags |= KBD_FLAGS_EXTENDED;
            return freerdp_input_send_keyboard_event(input, keyboard_flags, (UINT8)value);
        }
        case INPUT_UNICODE_KEY:
        {
            // value 低 16 位是 UTF-16 码元：BMP 之外的字（如 emoji）由上层拆成代理对分两次发。
            const UINT16 code = (UINT16)((uint32_t)value & 0xFFFFU);
            const UINT16 keyboard_flags = (flags & 1U) ? KBD_FLAGS_DOWN : KBD_FLAGS_RELEASE;
            return freerdp_input_send_unicode_keyboard_event(input, keyboard_flags, code);
        }
        case INPUT_CLIPBOARD_SET:
        {
            helperContext* helper = (helperContext*)context;
            const uint32_t declared = (uint32_t)value;
            if (declared > CLIPBOARD_TEXT_MAX_BYTES)
            {
                // 超限时也要把声明的字节读干净再丢弃，否则残包会被当成下一个事件，事件流永久错位。
                fprintf(stderr, "FreeRDP helper 本机剪贴板文本超限（%u 字节 > %u），已丢弃该事件\n",
                        declared, (uint32_t)CLIPBOARD_TEXT_MAX_BYTES);
                fflush(stderr);
                return discard_exact(declared);
            }

            BYTE* text = NULL;
            if (declared > 0)
            {
                text = malloc(declared);
                if (!text)
                {
                    fprintf(stderr, "FreeRDP helper 本机剪贴板文本分配失败（%u 字节），已丢弃该事件\n",
                            declared);
                    fflush(stderr);
                    return discard_exact(declared);
                }
                if (!read_exact(text, declared))
                {
                    free(text);
                    return FALSE;
                }
            }

            const BOOL stored = clipboard_store_local_text(helper, text, declared);
            free(text);
            if (!stored)
            {
                fprintf(stderr, "FreeRDP helper 本机剪贴板文本保存失败（%u 字节）\n", declared);
                fflush(stderr);
                return TRUE;
            }

            // 本机剪贴板变了就通知服务器；用户在远端粘贴时它会来索取这份文本。
            clipboard_send_local_format_list(helper);
            return TRUE;
        }
        case INPUT_CLIPBOARD_RESEND:
            clipboard_force_resend((helperContext*)context);
            return TRUE;
        case INPUT_RESIZE:
        {
            // type=9 没有尾部负载，事件本身已经被读完，非法值也不会让事件流错位。
            // 越界只打一行警告并忽略：既不改尺寸，也不上报"不支持"（这是上层的输入错误）。
            if ((raw_x < RESIZE_MIN_WIDTH) || (raw_x > RESIZE_MAX_WIDTH) ||
                (raw_y < RESIZE_MIN_HEIGHT) || (raw_y > RESIZE_MAX_HEIGHT))
            {
                fprintf(stderr,
                        "FreeRDP helper 动态分辨率请求越界（%ux%u，允许 %ux%u ~ %ux%u），已忽略\n",
                        raw_x, raw_y, RESIZE_MIN_WIDTH, RESIZE_MIN_HEIGHT, RESIZE_MAX_WIDTH,
                        RESIZE_MAX_HEIGHT);
                fflush(stderr);
                return TRUE;
            }
            return helper_request_resize(context, raw_x, raw_y);
        }
        case INPUT_CLOSE:
            // 与 EOF / 父进程消失走同一条干净退出路径：断开连接、释放上下文、退出码 0。
            helper_request_exit("close-event");
            return FALSE;
        default:
            return TRUE;
    }
}

static int run_client(rdpContext* context)
{
    freerdp* instance = context->instance;
    if (!freerdp_connect(instance))
    {
        // 看门狗为了回收孤儿会主动中止连接（freerdp_abort_connect_context）：
        // 这不是"连接失败"，而是"父进程没了、我们该收工了"，仍然按退出码 0 + 退出契约行处理。
        if (helper_exit_reason)
        {
            helper_report_exit();
            return 0;
        }
        // 证书被拒绝时必须返回稳定的非零退出码：上层据此判断「等待用户决定」而不是普通失败，
        // 而 freerdp_get_last_error 在证书校验失败时并不保证非零。
        if (certificate_rejected)
            return EXIT_CERTIFICATE_REJECTED;
        const int status = (int)freerdp_get_last_error(context);
        return status != 0 ? status : 1;
    }

    HANDLE input_event = CreateFileDescriptorEvent(NULL, FALSE, FALSE, fileno(stdin), WINPR_FD_READ);
    if (!input_event)
        return -1;

    // 到这里虚拟通道已经全部 post connect 完成：服务器没给剪贴板通道时留一条诊断，
    // 免得画面/输入正常却让人以为"剪贴板应该能用"。
    if (!((helperContext*)context)->cliprdr)
    {
        fprintf(stderr, "FreeRDP helper 本次会话没有剪贴板通道（服务器未分配 cliprdr）\n");
        fflush(stderr);
    }

    // 父进程检查按墙钟节流，每秒最多一次 getppid()。
    // 注意不能只在 WaitForMultipleObjects 返回 WAIT_TIMEOUT 时检查：实测 FreeRDP 的部分事件句柄
    // 会持续保持就绪（真机上等待会立刻返回、几乎从不超时），只在超时分支里查等于永远不查。
    const DWORD parent_check_ms = 1000;
    UINT64 last_parent_check = GetTickCount64();

    // 慢链路保护：握手本身可能比降级窗口还久（例如 TLS/MCS 就花了 6 秒），
    // 这时"从 PostConnect 起算"已经到期，但首帧往往立刻就到。主循环刚开始时若已到期，
    // 再宽限 1 秒，避免把"连得慢"误判成"开了动态分辨率就没画面"（后者才是要降级的真问题）。
    {
        helperContext* helper = (helperContext*)context;
        if (helper->fallback_armed && (helper->frames_output == 0) &&
            (GetTickCount64() >= helper->fallback_deadline))
            helper->fallback_deadline = GetTickCount64() + 1000;
    }

    while (!freerdp_shall_disconnect_context(context))
    {
        HANDLE handles[MAXIMUM_WAIT_OBJECTS] = { 0 };
        DWORD count = 0;
        handles[count++] = input_event;
        const DWORD rdp_count =
            freerdp_get_event_handles(context, &handles[count], MAXIMUM_WAIT_OBJECTS - count);
        if (rdp_count == 0)
        {
            helper_request_exit("remote-disconnect");
            break;
        }
        count += rdp_count;

        const DWORD status = WaitForMultipleObjects(count, handles, FALSE, parent_check_ms);
        if (status == WAIT_FAILED)
        {
            helper_request_exit("wait-failed");
            break;
        }

        // 被 launchd/init 收养（ppid==1）= 上层已经死了；即使 stdin 还开着也要自己收工，
        // 否则会永久占着服务器上的 RDP 会话。
        const UINT64 now = GetTickCount64();
        if ((now - last_parent_check) >= parent_check_ms)
        {
            last_parent_check = now;
            if (helper_parent_gone())
            {
                helper_request_exit("parent-gone");
                break;
            }
        }

        if ((status == WAIT_OBJECT_0) && !process_input(context))
            break;
        if (!freerdp_check_event_handles(context))
        {
            helper_request_exit("remote-disconnect");
            break;
        }
        // 尺寸请求的回应只会从 FreeRDP 的事件里来（Reactivate-All → DesktopResize 回调），
        // 所以放在事件处理之后检查：本轮刚处理完的回应不会被误判成超时。
        helper_check_resize_timeout(context);
        // "开了动态分辨率却一帧都没有" → 打契约行 + 干净退出（退出码 6）。
        if (helper_check_fallback_no_frames(context))
        {
            helper_request_exit("dynamic-resolution-fallback");
            break;
        }
        if (((helperContext*)context)->output_failed)
        {
            // 帧写不出去（上层已经不在读管道）说明没人要这个会话了，同样收工。
            helper_request_exit("remote-disconnect");
            break;
        }
    }

    CloseHandle(input_event);
    freerdp_disconnect(instance);
    // 统一在这里上报退出原因：stdout 只能是帧，所以走 stderr。
    helper_report_exit();
    // 动态分辨率自我降级要用稳定的退出码 6：上层据此记住"这台主机别开动态分辨率"。
    return ((helperContext*)context)->fallback_fired ? EXIT_DYNAMIC_RESOLUTION_FALLBACK : 0;
}

static void free_arguments(char** arguments, int count)
{
    for (int index = 0; index < count; index++)
        free(arguments[index]);
    free(arguments);
}

static char** read_arguments(int* count)
{
    size_t capacity = 16;
    char** arguments = calloc(capacity, sizeof(char*));
    if (!arguments)
        return NULL;
    arguments[0] = strdup("qterm-freerdp-helper");
    *count = 1;

    char* line = NULL;
    size_t line_capacity = 0;
    while (getline(&line, &line_capacity, stdin) >= 0)
    {
        const size_t length = strcspn(line, "\r\n");
        line[length] = '\0';
        if (length == 0)
            break;
        if ((size_t)*count == capacity)
        {
            capacity *= 2;
            char** expanded = realloc(arguments, capacity * sizeof(char*));
            if (!expanded)
            {
                free(line);
                free_arguments(arguments, *count);
                return NULL;
            }
            arguments = expanded;
        }
        arguments[*count] = strdup(line);
        if (!arguments[*count])
        {
            free(line);
            free_arguments(arguments, *count);
            return NULL;
        }
        (*count)++;
    }
    free(line);
    return arguments;
}

int main(void)
{
    // 避免读取参数时预取后续二进制输入事件。
    setvbuf(stdin, NULL, _IONBF, 0);
    setvbuf(stdout, NULL, _IONBF, 0);

    // stdout 是二进制帧通道，FreeRDP/WinPR 的所有日志必须固定到 stderr。
    wLog* root_log = WLog_GetRoot();
    if (!root_log || !WLog_SetLogAppenderType(root_log, WLOG_APPENDER_CONSOLE) ||
        !WLog_ConfigureAppender(WLog_GetLogAppender(root_log), "outputstream", "stderr"))
        return 1;

    int argument_count = 0;
    char** arguments = read_arguments(&argument_count);
    if (!arguments)
        return 2;

    RDP_CLIENT_ENTRY_POINTS entry = { 0 };
    entry.Version = RDP_CLIENT_INTERFACE_VERSION;
    entry.Size = sizeof(RDP_CLIENT_ENTRY_POINTS_V1);
    entry.ContextSize = sizeof(helperContext);
    entry.ClientNew = helper_client_new;

    rdpContext* context = freerdp_client_context_new(&entry);
    if (!context)
    {
        free_arguments(arguments, argument_count);
        return 3;
    }

    // 兜底看门狗从创建上下文这一刻就开工：它要覆盖"卡在连接握手里"的整个阶段。
    // 线程起不来不致命（主循环仍会检查），但必须留下诊断，否则孤儿回收会静默失效。
    InterlockedExchange(&helper_watchdog_stop, 0);
    HANDLE watchdog = helper_watchdog_start(context);
    if (!watchdog)
    {
        fprintf(stderr, "FreeRDP helper 看门狗线程创建失败，孤儿回收只依赖主循环\n");
        fflush(stderr);
    }

    const int parse_status = freerdp_client_settings_parse_command_line(
        context->settings, argument_count, arguments, FALSE);
    // 参数解析完成后尽快清理包含密码的临时副本。
    free_arguments(arguments, argument_count);
    if (parse_status != 0)
    {
        fprintf(stderr, "FreeRDP helper 参数解析失败: %d\n", parse_status);
        // 参数解析失败也要先停看门狗：它仍持有 context 指针，直接释放会造成
        // 看门狗线程访问已释放内存（尤其是上层同时关闭 stdin 的异常路径）。
        if (!helper_watchdog_stop_and_join(watchdog))
        {
            fprintf(stderr, "FreeRDP helper 看门狗线程未能退出，安全结束进程\n");
            fflush(NULL);
            _exit(4);
        }
        freerdp_client_context_free(context);
        return 4;
    }

    // 动态分辨率默认开启（QTERM_RDP_DYNAMIC_RESOLUTION=0 关闭）：启动时打一行可读诊断，
    // 让上层/用户知道当前取值，也避免"设了变量却因为写错值而静默生效/失效"。
    helperContext* helper = (helperContext*)context;
    helper->dynamic_resolution_enabled = helper_dynamic_resolution_enabled();
    fprintf(stderr, "动态分辨率：%s\n",
            helper->dynamic_resolution_enabled ? "已启用（" DYNAMIC_RESOLUTION_ENV "=0 可禁用）"
                                               : "已禁用（" DYNAMIC_RESOLUTION_ENV "=0）");
    // 剪贴板同样打一行可读诊断：真机排查"粘贴没反应"时，第一件要确认的就是
    // 这次会话到底挂了没有、方向是什么。
    helper->clipboard_feature_mask = helper_clipboard_feature_mask(&helper->clipboard_mode_label,
                                                                   &helper->clipboard_content_label);
    fprintf(stderr, "剪贴板：方向=%s 内容=%s\n", helper->clipboard_mode_label,
            helper->clipboard_content_label);
    fflush(stderr);

    const int result = run_client(context);
    // 先停掉看门狗再释放上下文：它会在另一个线程里访问 context（abort 连接）。
    if (!helper_watchdog_stop_and_join(watchdog))
    {
        fprintf(stderr, "FreeRDP helper 看门狗线程未能退出，安全结束进程\n");
        fflush(NULL);
        _exit(result);
    }
    freerdp_client_context_free(context);
    return result;
}
