# 第一階段：構建階段
FROM golang:1.24-alpine AS build

# 安裝基本依賴
RUN apk --no-cache add git

# 設置工作目錄
WORKDIR /app

# 複製 go.mod 和 go.sum (如果有的話)
COPY go.mod ./
COPY go.sum ./

# 下載依賴
RUN go mod download

# 複製源代碼
COPY *.go ./
COPY home.html ./

# 編譯應用程序
RUN CGO_ENABLED=0 GOOS=linux go build -o iogame .

# 第二階段：運行階段
FROM alpine:latest

# 添加必要的運行時依賴
RUN apk --no-cache add ca-certificates

# 設置工作目錄
WORKDIR /app

# 從構建階段複製編譯好的應用程序和靜態文件
COPY --from=build /app/iogame .
COPY --from=build /app/home.html .

# 設置環境變量
ENV PORT=8080

# 暴露端口
EXPOSE 8080

# 運行應用程序
CMD ["./iogame"]    