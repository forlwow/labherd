# Controller 镜像。只打包已经构建好的静态二进制（前端已嵌入），不在镜像里编译。
# CI 中以 out/ 为构建上下文，见 .github/workflows/release.yml。
# 本地构建：
#   CGO_ENABLED=0 make release
#   docker build -f build/controller.Dockerfile --build-arg BINARY=labherd-controller bin/
FROM gcr.io/distroless/static-debian12:nonroot

ARG BINARY=labherd-controller-linux-amd64
COPY ${BINARY} /usr/local/bin/labherd-controller

# 9090：Agent 的 gRPC 连接；8080：网页与 HTTP 接口
EXPOSE 9090 8080
ENTRYPOINT ["/usr/local/bin/labherd-controller"]
