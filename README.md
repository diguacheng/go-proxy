# 内网穿透工具

- 多协程快速响应
- 心跳包维护连接
- client断开自动重连

## 说明
- server端： 具有公网地址的服务器
- client端： 需要内网穿透的主机

## 使用

server端, 默认本地为5200端口
```bash
./server -l 5200 -r 3333
```

client端
```bash
./client -l 8080 -r 3333
```

用户访问 `服务IP地址:5200` 即可访问到 内网中的 `8080`端口程序