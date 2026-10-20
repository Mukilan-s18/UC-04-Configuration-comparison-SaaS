FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY bin/saas-server /app/saas-server
COPY web /app/web
EXPOSE 8080
ENV PORT=8080 ENVIRONMENT=production
ENTRYPOINT ["/app/saas-server"]
