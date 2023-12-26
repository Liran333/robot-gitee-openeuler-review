FROM openeuler/openeuler:23.03 as BUILDER
RUN dnf update -y && \
    dnf install -y golang && \
    go env -w GOPROXY=https://goproxy.cn,direct

MAINTAINER zengchen1024<chenzeng765@gmail.com>

# build binary
WORKDIR /go/src/github.com/opensourceways/robot-gitee-openeuler-review
COPY . .
RUN GO111MODULE=on CGO_ENABLED=0 go build -a -o robot-gitee-openeuler-review -buildmode=pie --ldflags "-s -linkmode 'external' -extldflags '-Wl,-z,now'" .

# copy binary config and utils
FROM openeuler/openeuler:22.03
RUN dnf -y update && \
    dnf in -y shadow && \
    dnf remove -y gdb-gdbserver && \
    groupadd -g 1000 openeuler-review && \
    useradd -u 1000 -g openeuler-review -s /sbin/nologin -m openeuler-review && \
    echo > /etc/issue && echo > /etc/issue.net && echo > /etc/motd && \
    mkdir /home/openeuler-review -p && \
    chmod 700 /home/openeuler-review && \
    chown openeuler-review:openeuler-review /home/openeuler-review && \
    echo 'set +o history' >> /root/.bashrc && \
    sed -i 's/^PASS_MAX_DAYS.*/PASS_MAX_DAYS   90/' /etc/login.defs && \
    rm -rf /tmp/*

USER openeuler-review

WORKDIR /opt/app

COPY  --chown=openeuler-review --from=BUILDER /go/src/github.com/opensourceways/robot-gitee-openeuler-review/robot-gitee-openeuler-review /opt/app/robot-gitee-openeuler-review

RUN chmod 550 /opt/app/robot-gitee-openeuler-review && \
    echo "umask 027" >> /home/openeuler-review/.bashrc && \
    echo 'set +o history' >> /home/openeuler-review/.bashrc

ENTRYPOINT ["/opt/app/robot-gitee-openeuler-review"]
