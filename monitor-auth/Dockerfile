FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/monitor-auth .

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/monitor-auth /monitor-auth
EXPOSE 8080
ENTRYPOINT ["/monitor-auth"]
