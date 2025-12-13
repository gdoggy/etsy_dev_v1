# 申请 Etsy 开发者账号
- 1个开发账号（个人授权）最多可以接入 5 个店铺
## 1. 准备环境：
- 开启 指纹浏览器 或者使用干净的 Chrome Profile + 代理 IP 来访问 Etsy。不要直接用平时购物的 IP。

## 2. 访问注册：

- https://www.etsy.com/developers/register。

## 3.开启 2FA (双重验证)：

- 硬性规定，需要下载一个 Authenticator App (如 Google Authenticator 或 Microsoft Authenticator) 绑定账号。不绑无法创建 App。

## 4.创建 App (获取 Key)：

- 填写 App 信息时，Website URL 暂时可以填 https://www.example.com。

- Callback URL (重要)：填 http://localhost:8080/api/auth/callback。

## 5.获取结果：

- 提交后得到一个 Keystring (App Key) 和 Shared Secret