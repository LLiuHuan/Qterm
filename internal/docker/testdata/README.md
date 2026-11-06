# docker 解析 fixture 说明

本目录是 `internal/docker` 解析测试的输入样本。**测试运行时不需要 docker**：这些是抓取后的静态文件。

| 文件                        | 来源                                                                                                                                                                                                                             | 脱敏/改动                                                                                                                                                                                                     |
| --------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `containers.ps.txt`         | **真实抓取**：`docker ps -a --format '<tab 投影模板>'`（Docker 29.3.1 / Docker Desktop 4.68.0，macOS）                                                                                                                           | 无（容器名、镜像名、端口均为本机开发容器，不含秘密）                                                                                                                                                          |
| `container.inspect.json`    | **真实抓取后裁剪**：`docker inspect 3e7ece12ef99` 只保留解析用到的键（`Id`/`Created`/`State`/`Name`/`Mounts`/`Config`/`NetworkSettings`），保留一个额外键（`Path`/`Args`）以证明解析器容忍未知字段                               | 挂载源路径改为 `/Users/demo/...`；两条 `Mounts` 中第二条（volume）为**合成**项；三个 `com.docker.compose.*` 标签为**合成**项（本机没有 compose 项目）                                                         |
| `version.json`              | **真实抓取后裁剪**：`docker version --format '{{json .}}'`，只保留 `Client`/`Server` 的版本相关键                                                                                                                                | 无                                                                                                                                                                                                            |
| `containers.compose.ps.txt` | **全合成**：覆盖多 compose 项目、独立容器、空列、坏行（段数不足）、`working_dir` 末尾带 tab 的极端情形                                                                                                                           | 全合成                                                                                                                                                                                                        |
| `images.ndjson`             | **真实抓取后裁剪**：`docker images --no-trunc --format '{{json .}}'`（Docker 29.3.1 / Docker Desktop 4.68.0，macOS），保留 8 行                                                                                                  | 仓库名脱敏（`pushpenderindia/bookworm-vnc`→`demo/bookworm-vnc`、`kasmweb/...`→`demo/...`、`minio/mc`→`demo/mc`），字段与值形态不变；**第 4 行悬空镜像是合成项**（本机没有悬空镜像，按真实 `<none>` 形态构造） |
| `dangling.ndjson`           | **真实抓取**：`docker images -f dangling=true --no-trunc --format '{{json .}}'`（Docker 29.3.1 / Docker Desktop 4.68.0，macOS），3 行                                                                                            | 无（悬空镜像名称、ID、大小均为本机现状，不含秘密）；用于证明悬空镜像不在默认 `docker images` 输出里，必须单独查询再合并                                                                                       |
| `volumes.ndjson`            | **真实抓取 + 2 行合成**：`docker volume ls --format '{{json .}}'`，前 3 行真实（2 个匿名卷 + buildx 卷）                                                                                                                         | `qterm-demo-cache` 为**合成**项（带 compose 标签，供 in-use 交叉引用）；`qterm-legacy-pgdata` 为**合成**项且**故意不带 `Mountpoint` 键**（旧版 CLI 不输出该字段，解析必须容忍）                               |
| `networks.ndjson`           | **真实抓取 + 5 行合成**：`/usr/local/bin/docker network ls --no-trunc --format '{{json .}}'`（Docker 29.3.1 / Docker Desktop 4.68.0，macOS），前 5 行逐字保留（bridge / compose 网络 / host / none / 用户网络；ID 为完整 64 位） | 第 6–10 行为**合成**项：网络名含逗号（`demo,net`）与含空格（`demo net`）各 1 行、缺 `ID`/`Driver`/`Scope` 键 1 行、截断的坏 JSON 1 行、`Name` 为空 1 行；真实行里的 compose 标签值原样保留（不含秘密）        |
| `inspect.mounts.txt`        | **真实抓取后裁剪 + 1 行合成**：`docker inspect --format '{{.Image}}\t{{json .Mounts}}' <19 个容器>`，保留 5 行（2 行空挂载、1 行纯 bind、1 行纯 volume、1 行双 bind）                                                            | 挂载源路径 `/Users/liuhuan/…`→`/Users/demo/…`；**末行（bind + 具名卷 + 匿名卷混排）为合成项**，覆盖卷 in-use 的三种形态                                                                                       |

## 为什么 ps 用 tab 投影而不是 `{{json .}}`

真实镜像标签的值里**含逗号**（例如 `org.opencontainers.image.description = "Bookworm VNC server, noVNC web interface"`），
而 `docker ps --format '{{json .}}'` 的 `.Labels` 是 `k=v,k2=v2` 字符串，按逗号切分必然切坏。
因此容器列表改用 tab 投影模板，每个字段一列，且 `com.docker.compose.*` 三个标签由模板的
`{{.Label "..."}}` 直接取，不再解析标签字符串（见 ADR-019 §2.2/§2.3 的修订）。

NDJSON 解析器仍然保留（`parseNDJSONObjects`），供 P1 的镜像/卷/网络列表使用。

## 为什么 in-use 投影的 `\t` 要两种形态都容忍

`docker ps --format '{{.ID}}\t{{.Names}}'` 实测把 `\t` **展开成真 TAB**，而
`docker inspect --format '{{.Image}}\t{{json .Mounts}}'` 在 Docker 29.3.1 上**原样输出
字面 `\t` 两个字符**（`od -c` 核对：`\` 与 `t`，不是 0x09）。两条命令的模板处理并不一致。

因此 `parseInUseLine` 接受「真 TAB」与「字面 `\t`」两种分隔符，取更靠前的那个：镜像引用
只可能是 `sha256:<hex>`、不含反斜杠，所以第一个命中的分隔符必然是投影分隔符，不会切进
JSON 内容（JSON 字符串里的制表符会被转义成 `\t`，只可能出现在分隔符**之后**）。
如果只认其中一种，换一个 CLI 版本就会把整份 in-use 数据判成坏行——列表还能显示，但
IN USE / UNUSED 分组会全错。

## 为什么网络 in-use 不能按分隔符切分

网络 in-use 用的是另一条投影：`docker inspect --format '{{json .NetworkSettings.Networks}}'`，
每个容器输出一行 JSON 对象，例如 `{"demo,net":{…},"demo net":{…}}`。Docker 29.3.1 实测
接受逗号、空格、斜杠、冒号与以 `-` 开头的网络名，因此**只能把整行当作 JSON 对象解析并取
map 的键**；任何 `Split(",")` / `Split(" ")` 都会把合法名字切坏。`networks.ndjson` 里的
`demo,net` 与 `demo net` 两行就是这条规则的回归样本。名称只用于展示与 in-use 精确匹配；
**删除网络走 ID**（cli 会规范化名称，按名称删可能 not found），因此这些名字不需要、也不允许
进入删除命令的 argv。
