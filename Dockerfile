FROM gcr.io/distroless/base
COPY out/main /main
ENTRYPOINT [ "/main" ]
