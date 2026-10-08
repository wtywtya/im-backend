# Go 即时通信后端

基于 Go、Gin、WebSocket、MySQL 和 Redis 开发的即时通信后端学习项目。

## 功能

- 用户注册与登录，使用 bcrypt 保存密码哈希。
- 使用 JWT 验证用户身份。
- 通过 WebSocket 发送和接收消息。
- 将消息保存到 MySQL，支持分页查询历史消息。
- 使用 Redis Pub/Sub 实现跨服务实例的消息转发。

## 技术栈

Go、Gin、GORM、MySQL、Redis、WebSocket、JWT。

## 运行配置

先启动 MySQL 和 Redis，并确保 MySQL 中已创建 `im_db` 数据库。程序会自动创建或更新数据表，但不会创建数据库本身。

程序必须从环境变量 `MYSQL_DSN` 读取数据库连接信息，不再使用源码中的默认密码。

服务还必须配置 `JWT_SECRET`，用于签发和验证登录 Token，源码不包含默认密钥。两个实例必须使用同一密钥。首次在自己的 Windows 用户下运行时，可执行以下命令生成并保存一次（已有密钥会保留，不要为了重启服务反复更换密钥）：

```powershell
$env:JWT_SECRET = [Environment]::GetEnvironmentVariable('JWT_SECRET', 'User')
if ([string]::IsNullOrWhiteSpace($env:JWT_SECRET)) {
    $jwtBytes = New-Object byte[] 32
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($jwtBytes) } finally { $rng.Dispose() }
    $env:JWT_SECRET = [Convert]::ToBase64String($jwtBytes)
    [Environment]::SetEnvironmentVariable('JWT_SECRET', $env:JWT_SECRET, 'User')
}
```

在启动服务的 PowerShell 终端配置（将占位内容换成自己的账号和密码）：

```powershell
$env:MYSQL_DSN = '数据库用户名:数据库密码@tcp(127.0.0.1:3306)/im_db?charset=utf8mb4&parseTime=True&loc=Local'
go run . -port 8080
```

如果本机已经保存了 Windows 用户环境变量 `MYSQL_DSN`，已有的 VS Code 终端可以读取保存的值，无需重新输入密码：

```powershell
$env:MYSQL_DSN = [Environment]::GetEnvironmentVariable('MYSQL_DSN', 'User')
$env:JWT_SECRET = [Environment]::GetEnvironmentVariable('JWT_SECRET', 'User')
go run . -port 8080
```

每个用于启动服务或压测的新终端都需要具备这个变量。启动第二个实例时使用 `go run . -port 8081`。环境变量不是加密存储，请勿将实际连接密码写入文档、源码或公开截图。

启动服务的终端还需要读取 `JWT_SECRET`。更换密钥后应重启所有实例；旧 Token 会失效，需要重新登录，账号和消息记录不受影响。压测工具通过登录接口获取新 Token，无需知道签名密钥。

`REDIS_ADDR` 可选，默认值为 `localhost:6379`。

## 本地测试

在同一台电脑上启动两个服务实例，分别监听 8080 和 8081 端口。
发送端连接 8080，接收端连接 8081，通过 Redis 转发消息。

使用 100 个并发发送连接，每个连接发送 20 条消息，重复测试三轮：

- 共发送 6,000 条消息，全部收到并写入 MySQL。
- 未观察到重复接收和客户端发送错误。
- 每轮总耗时约 352～389 毫秒。

以上属于本机短时突发负载测试，不代表系统极限或长期稳定性。
