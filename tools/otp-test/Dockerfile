FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        libpam-google-authenticator \
        openssh-server \
        python3 \
        qrencode \
    && useradd --create-home --shell /bin/bash qtermotp \
    && printf '%s\n' 'qtermotp:qterm-otp-password' | chpasswd \
    && mkdir -p /run/sshd /opt/otp-test \
    && chmod 0755 /run/sshd \
    && ssh-keygen -A

COPY sshd_config /etc/ssh/sshd_config
COPY pam-sshd /etc/pam.d/sshd
COPY google_authenticator /home/qtermotp/.google_authenticator
COPY totp.py /opt/otp-test/totp.py
COPY entrypoint.sh /usr/local/sbin/otp-test-entrypoint

RUN chown qtermotp:qtermotp /home/qtermotp/.google_authenticator \
    && chmod 0600 /home/qtermotp/.google_authenticator \
    && chmod 0755 /usr/local/sbin/otp-test-entrypoint /opt/otp-test/totp.py \
    && printf '%s\n' 'otpauth://totp/Qterm%20OTP:qtermotp?secret=JBSWY3DPEHPK3PXP&issuer=Qterm%20OTP&algorithm=SHA1&digits=6&period=30' > /opt/otp-test/otpauth.txt \
    && qrencode -l M -o /opt/otp-test/otpauth.png "$(cat /opt/otp-test/otpauth.txt)"

EXPOSE 22

ENTRYPOINT ["/usr/local/sbin/otp-test-entrypoint"]
