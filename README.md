# huashunxinan

基于 mDNS/DNS-SD 协议的网段资产探测 CLI 工具。输入 IP 网段与端口范围，自动发现该网段内所有广播 mDNS 服务的设备，并输出深度识别的 banner 信息。

---

## 目录

- [功能特性](#功能特性)
- [安装](#安装)
- [运行方式](#运行方式)
- [快速上手](#快速上手)
- [参数说明](#参数说明)
- [输出格式](#输出格式)
- [示例输出](#示例输出)
- [架构设计](#架构设计)
- [mDNS 发现原理](#mdns-发现原理)
- [文件结构](#文件结构)
- [常见设备类型参考](#常见设备类型参考)

---

## 功能特性

- **主动探测**：向指定网段内每个 IP 的 UDP 5353 端口发送单播 DNS-SD 查询
- **全服务枚举**：通过 `_services._dns-sd._udp.local.` 枚举目标设备上所有注册的 mDNS 服务类型
- **深度 banner**：提取每个服务实例的 SRV 记录（端口、主机名）和 TXT 记录（元数据键值对）
- **地址解析**：自动从 mDNS 响应的 Additional 段缓存并关联 A / AAAA 记录
- **端口过滤**：支持按端口范围或端口列表过滤最终输出
- **高并发扫描**：内置 worker pool，可控制并发数量和单 host 超时
- **IPv4 / IPv6 双栈**：同时展示目标设备的 IPv4 和 IPv6 链路本地地址

---

## 安装

**前置要求**：Go 1.21+

```bash
git clone https://github.com/jiaguoliang/huashunxinan.git
cd huashunxinan
```

---

## 运行方式

项目提供三种运行方式，按使用场景选择。

### 方式一：直接运行（无需编译，开发调试推荐）

```bash
go run . -cidr 192.168.1.0/24
```

`go run .` 会编译当前目录所有 `.go` 文件并立即执行，无需手动 `build`，适合开发期快速验证。

### 方式二：编译后运行（生产部署推荐）

```bash
# 编译为当前平台可执行文件
go build -o huashunxinan .

# 运行
./huashunxinan -cidr 192.168.1.0/24
```

编译产物为单个二进制文件，无任何外部依赖，可直接复制到其他同架构机器运行。

**交叉编译**（在 macOS 上编译 Linux amd64 版本）：

```bash
GOOS=linux GOARCH=amd64 go build -o huashunxinan-linux-amd64 .
```

### 方式三：全局安装

```bash
go install github.com/jiaguoliang/huashunxinan@latest

# 安装后可在任意目录直接调用（需 $GOPATH/bin 在 PATH 中）
huashunxinan -cidr 192.168.1.0/24
```

### 查看帮助

不传参数运行时输出用法说明：

```
$ ./huashunxinan
Usage: huashunxinan -cidr <CIDR> [-ports <range>] [-timeout <sec>] [-workers <n>]
  -cidr     required  192.168.1.0/24
  -ports    optional  1-65535 (default), 80,443,5000, or 1-1000
  -timeout  optional  per-host timeout seconds (default 3)
  -workers  optional  concurrent goroutines   (default 50)
```

---

## 快速上手

```bash
# 扫描整个 C 段，全端口
./huashunxinan -cidr 192.168.1.0/24

# 用 go run 直接运行（等价）
go run . -cidr 192.168.1.0/24

# 扫描指定端口范围
./huashunxinan -cidr 192.168.1.0/24 -ports 1-1000

# 扫描离散端口列表
./huashunxinan -cidr 192.168.1.0/24 -ports 22,80,443,445,548,5000

# 组合范围与离散端口
./huashunxinan -cidr 10.0.0.0/24 -ports 1-1024,5000,8080

# 扫描单个 IP
./huashunxinan -cidr 192.168.1.100/32

# 加快扫描速度（增大并发、减小超时）
./huashunxinan -cidr 192.168.1.0/24 -timeout 1 -workers 100
```

---

## 参数说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `-cidr` | string | **必填** | 目标网段，CIDR 格式，例如 `192.168.1.0/24`、`10.0.0.1/32` |
| `-ports` | string | `1-65535` | 端口过滤范围，支持区间 `1-1000`、离散值 `80,443`、混合 `1-1024,8080` |
| `-timeout` | int | `3` | 每个 host 的 UDP 查询超时时间（秒） |
| `-workers` | int | `50` | 并发扫描的 goroutine 数量 |

**注意**：`-ports` 过滤的是最终服务的 TCP/UDP 端口（SRV 记录中的端口），而非 mDNS 使用的 UDP 5353 端口。端口为 0 的服务（如 `device-info`）不受端口过滤限制，始终输出。

---

## 输出格式

每个发现了 mDNS 服务的 host 输出一个块，格式如下：

```
# Host: <IP>
services:
<port>/<proto> <service-type>:
Name=<实例名>
IPv4=<IPv4地址>
IPv6=<IPv6链路本地地址>
Hostname=<主机名.local>
TTL=<TTL值>
[TXT键值对，每行一条]

<service-type>:          ← 端口为0时省略端口前缀
Name=<实例名>
...

answers:
PTR:
<_service._tcp.local>    ← 该host上发现的所有服务类型
...
```

### 字段说明

| 字段 | 来源 | 含义 |
|------|------|------|
| `<port>/<proto>` | SRV 记录 | 服务监听端口和协议（tcp/udp） |
| `<service-type>` | PTR 记录名 | 服务类型短名称，如 `http`、`smb`、`workstation` |
| `Name` | PTR 记录值（实例名） | 设备上注册的服务实例名，含设备名或 MAC |
| `IPv4` | A 记录 | 设备 IPv4 地址 |
| `IPv6` | AAAA 记录 | 设备 IPv6 链路本地地址（`fe80::` 开头） |
| `Hostname` | SRV 记录 target | 设备的 `.local` 主机名 |
| `TTL` | DNS 记录 TTL | 记录存活时间（秒） |
| TXT 行 | TXT 记录 | 服务元数据，各字段由设备厂商定义 |
| `PTR` 段 | 枚举结果 | 该 host 上所有已注册的 mDNS 服务类型列表 |

---

## 示例输出

以下均为在真实局域网环境中执行的实际输出，stderr（扫描进度）与 stdout（结果）分开显示。

---

### 示例 1：扫描整段 /24，无端口过滤

**命令：**

```bash
$ go run . -cidr 10.8.163.0/24 -timeout 1 -workers 50
```

**终端输出（stderr 进度 + stdout 结果）：**

```
Scanning 254 hosts on port range "1-65535" ...

# Host: 10.8.163.5
services:
22/tcp sftp-ssh:
Name=tal的MacBook Pro
IPv4=10.8.163.5
IPv6=fe80::8be:e7c3:9202:27e
Hostname=taldeMacBook-Pro.local
TTL=10
22/tcp ssh:
Name=tal的MacBook Pro
IPv4=10.8.163.5
IPv6=fe80::8be:e7c3:9202:27e
Hostname=taldeMacBook-Pro.local
TTL=10
answers:
PTR:
_sftp-ssh._tcp.local
_ssh._tcp.local

# Host: 10.8.163.9
services:
7000/tcp airplay:
Name=MacBook Pro
IPv4=10.8.163.9
IPv6=fe80::46a:5527:d24b:877f
Hostname=MacBook-Pro.local
TTL=10
act=2
acl=0
deviceid=F6:35:3D:3A:B4:0C
fex=1c9/St5PFzg2IQw
features=0x4A7FCFD5,0x38174FDE
rsf=0x8
flags=0x204
gid=AEF11A84-B4B4-4D86-AA6D-0B06E3AB1809
model=MacBookPro16,1
pi=5f2ff643-7682-4274-9a72-74ab06418fef
pk=4595bafe3c8f4c5c7a2d8ea6cd2042d067056e47fd258d13f81d528c147b3332
srcvers=860.7.1
7000/tcp raop:
Name=F6353D3AB40C@MacBook Pro
IPv4=10.8.163.9
IPv6=fe80::46a:5527:d24b:877f
Hostname=MacBook-Pro.local
TTL=10
cn=0,1,2,3
da=true
et=0,3,5
ft=0x4A7FCFD5,0x38174FDE
sf=0x204
md=0,1,2
am=MacBookPro16,1
pk=4595bafe3c8f4c5c7a2d8ea6cd2042d067056e47fd258d13f81d528c147b3332
tp=UDP
vn=65537
vs=860.7.1
vv=0
49464/tcp companion-link:
Name=MacBook Pro
IPv4=10.8.163.9
IPv6=fe80::46a:5527:d24b:877f
Hostname=MacBook-Pro.local
TTL=10
rpBA=1F:F5:34:A6:EB:9C
rpAD=d5cba82e1876
rpFl=0x20000
rpHN=ab94245ad66d
rpMac=0
rpVr=660.5.1
answers:
PTR:
_airplay._tcp.local
_companion-link._tcp.local
_raop._tcp.local

# Host: 10.8.163.16
services:
5900/tcp rfb:
Name=Yhh
IPv4=10.8.163.16
IPv6=fe80::1022:27e8:1f13:5803
Hostname=Yhh.local
TTL=10
7000/tcp airplay:
Name=Yhh
IPv4=10.8.163.16
IPv6=fe80::1022:27e8:1f13:5803
Hostname=Yhh.local
TTL=10
act=2
acl=0
deviceid=6E:44:C9:F6:B9:7A
features=0x4A7FCFD5,0x38174FDE
model=Mac16,1
pk=b5209106382c3c30da2c732ad5a2bd1e05ee7e9a167e3b064f8324a7c5bad6d4
srcvers=950.7.1
7000/tcp raop:
Name=6E44C9F6B97A@Yhh
IPv4=10.8.163.16
IPv6=fe80::1022:27e8:1f13:5803
Hostname=Yhh.local
TTL=10
cn=0,1,2,3
da=true
et=0,3,5
am=Mac16,1
pk=b5209106382c3c30da2c732ad5a2bd1e05ee7e9a167e3b064f8324a7c5bad6d4
vs=950.7.1
answers:
PTR:
_airplay._tcp.local
_raop._tcp.local
_rfb._tcp.local

# Host: 10.8.163.46
services:
56666/udp mi-connect:
Name={"nm":"客厅的小米电视","as":"[2, 16377]","ip":"71"}
IPv4=10.8.163.46
IPv6=fe80::6a39:43ff:fea4:346e
Hostname=Android.local
TTL=10
idHash=MTYw
dev=2
sec=2
apps=[2, 16377]
name=客厅的小米电视
version=65543
answers:
PTR:
_mi-connect._udp.local
```

---

### 示例 2：QNAP NAS 完整输出

**命令：**

```bash
$ go run . -cidr 192.168.1.50/32
```

**终端输出：**

```
Scanning 1 hosts on port range "1-65535" ...

# Host: 192.168.1.50
services:
9/tcp workstation:
Name=slw-nas [24:5e:be:69:a3:13]
IPv4=192.168.1.50
IPv6=fe80::265e:beff:fe69:a313
Hostname=slw-nas.local
TTL=10
445/tcp smb:
Name=slw-nas
IPv4=192.168.1.50
IPv6=fe80::265e:beff:fe69:a313
Hostname=slw-nas.local
TTL=10
548/tcp afpovertcp:
Name=slw-nas(AFP)
IPv4=192.168.1.50
IPv6=fe80::265e:beff:fe69:a313
Hostname=slw-nas.local
TTL=10
5000/tcp http:
Name=slw-nas
IPv4=192.168.1.50
IPv6=fe80::265e:beff:fe69:a313
Hostname=slw-nas.local
TTL=10
path=/
5000/tcp qdiscover:
Name=slw-nas
IPv4=192.168.1.50
IPv6=fe80::265e:beff:fe69:a313
Hostname=slw-nas.local
TTL=10
accessType=https,accessPort=86,model=TS-X64,displayModel=TS-464C,fwVer=5.2.9,fwBuildNum=20260214
device-info:
Name=slw-nas(AFP)
IPv4=192.168.1.50
IPv6=fe80::265e:beff:fe69:a313
Hostname=slw-nas.local
TTL=10
model=Xserve
answers:
PTR:
_afpovertcp._tcp.local
_device-info._tcp.local
_http._tcp.local
_qdiscover._tcp.local
_smb._tcp.local
_workstation._tcp.local
```

---

### 示例 3：按端口过滤，只看 SSH 和 Web 服务

**命令：**

```bash
$ go run . -cidr 10.8.163.0/24 -ports 22,80,443,5000 -timeout 1
```

**终端输出：**

```
Scanning 254 hosts on port range "22,80,443,5000" ...

# Host: 10.8.163.5
services:
22/tcp sftp-ssh:
Name=tal的MacBook Pro
IPv4=10.8.163.5
IPv6=fe80::8be:e7c3:9202:27e
Hostname=taldeMacBook-Pro.local
TTL=10
22/tcp ssh:
Name=tal的MacBook Pro
IPv4=10.8.163.5
IPv6=fe80::8be:e7c3:9202:27e
Hostname=taldeMacBook-Pro.local
TTL=10
answers:
PTR:
_sftp-ssh._tcp.local
_ssh._tcp.local
```

---

### 示例 4：目标无 mDNS 服务

**命令：**

```bash
$ go run . -cidr 10.8.163.200/32 -timeout 1
```

**终端输出：**

```
Scanning 1 hosts on port range "1-65535" ...
No mDNS services found.
```

---

### 示例 5：参数错误提示

```bash
$ go run . -cidr 999.999.0.0/24
Error: invalid CIDR "999.999.0.0/24": invalid CIDR address: 999.999.0.0/24

$ go run . -ports "abc-xyz"
Error: invalid port "abc"
```

---

## 架构设计

```
main.go
  └─ 解析 CLI 参数
  └─ expandCIDR()      → 生成 IP 列表
  └─ parsePortRange()  → 生成端口集合
  └─ Scanner.Scan()    → 并发扫描 → []*HostResult
  └─ PrintResults()    → 格式化输出

Scanner (scanner.go)
  ├─ worker pool (goroutine × workers)
  │    └─ MDNSQuerier.QueryHost(ip) → *HostResult
  └─ 端口过滤 + IP 排序

MDNSQuerier (mdns.go)
  ├─ Stage 1: query("_services._dns-sd._udp.local.", PTR)
  │              → 发现服务类型列表
  ├─ Stage 2: query("<_service._tcp.local.>", PTR) × N
  │              → 发现服务实例列表
  └─ Stage 3: query("<instance>", SRV) + query("<instance>", TXT) × M
                 → 解析端口、主机名、TXT banner
                 → 地址缓存 (addrCache) 关联 A/AAAA
```

### 并发模型

```
ips [254]
  │
  ▼
jobs channel ──► goroutine 1 ─┐
                goroutine 2 ─┤
                goroutine 3 ─┤─► results (mutex 保护)
                    ...      ─┘
                goroutine N ──┘
```

每个 goroutine 从 `jobs` channel 取 IP，完成后写入 `results`。`workers` 参数控制最大并发数，`timeout` 控制每个 UDP 查询的等待时间。

---

## mDNS 发现原理

mDNS（Multicast DNS，RFC 6762）与 DNS-SD（DNS Service Discovery，RFC 6763）配合使用，允许设备在无 DNS 服务器的局域网内发现彼此的服务。

### 标准多播流程

```
Client                          Device (224.0.0.251:5353)
  │                                      │
  │── PTR? _services._dns-sd._udp.local ─►│  (多播)
  │◄─ PTR: _http._tcp.local             ──│
  │◄─ PTR: _smb._tcp.local              ──│
  │                                      │
  │── PTR? _http._tcp.local ─────────────►│
  │◄─ PTR: nas._http._tcp.local         ──│
  │◄─ SRV: nas._http._tcp.local → :5000 ──│
  │◄─ TXT: path=/ ───────────────────────│
  │◄─  A:  nas.local → 192.168.1.50     ──│
```

### 本工具的单播主动探测

本工具不依赖多播监听，而是向每个目标 IP 的 UDP 5353 端口发送单播查询，主动触发响应：

```
Scanner                         Target IP:5353 (UDP)
  │                                      │
  │── UDP unicast: PTR? _services... ───►│
  │◄─ UDP response: PTR records ─────────│
  │                                      │
  │── UDP unicast: PTR? _http._tcp... ──►│
  │◄─ PTR + SRV + TXT + A (Additional) ─│
  │                                      │
  │── UDP unicast: SRV? <instance> ─────►│
  │◄─ SRV + TXT + A/AAAA ────────────────│
```

**地址缓存机制**：mDNS 响应常在 Additional 段附带 A/AAAA 记录。工具维护一个 `addrCache map[string]addrPair`，在每次收到响应时提取这些记录，避免额外的地址查询轮次。

### DNS 记录类型与用途

| 记录类型 | 用途 |
|----------|------|
| PTR | 服务类型枚举、服务实例列表 |
| SRV | 服务实例对应的端口号和主机名 |
| TXT | 服务元数据（路径、型号、固件版本等） |
| A | 主机名 → IPv4 地址 |
| AAAA | 主机名 → IPv6 地址 |

---

## 文件结构

```
.
├── main.go      CLI 入口：参数解析、组装流程
├── types.go     核心数据结构
│                  ServiceRecord  单个服务实例
│                  HostResult     单个 host 的全部发现结果
├── mdns.go      mDNS 查询层
│                  MDNSQuerier    单 host 三段式发现逻辑
│                  send()         UDP 单播 DNS 查询
│                  collectAddresses()  A/AAAA 地址缓存
│                  unescapeDNS()  DNS label 反转义
├── scanner.go   扫描协调层
│                  Scanner        并发 worker pool
│                  expandCIDR()   CIDR → IP 列表
│                  parsePortRange() 端口范围解析
└── output.go    输出格式化
                   PrintResults() 主输出入口
                   printService() 单服务块格式化
```

---

## 常见设备类型参考

| 服务类型 | 默认端口 | 常见设备 |
|----------|----------|----------|
| `_workstation._tcp` | 9 | Linux/macOS 工作站（含 MAC 地址） |
| `_ssh._tcp` | 22 | 支持 SSH 的设备 |
| `_sftp-ssh._tcp` | 22 | 支持 SFTP 的设备 |
| `_http._tcp` | 80/8080/5000 | Web 服务，NAS 管理界面 |
| `_https._tcp` | 443 | HTTPS Web 服务 |
| `_smb._tcp` | 445 | Samba 文件共享（Windows/NAS） |
| `_afpovertcp._tcp` | 548 | Apple 文件共享协议（AFP） |
| `_nfs._tcp` | 2049 | NFS 文件共享 |
| `_airplay._tcp` | 7000 | Apple AirPlay 投屏 |
| `_raop._tcp` | 7000 | Apple AirPlay 音频（含 MAC@Name） |
| `_companion-link._tcp` | 动态 | iPhone/iPad/Mac 联动 |
| `_qdiscover._tcp` | 5000 | QNAP NAS 发现协议（含固件信息） |
| `_device-info._tcp` | 0 | Apple 设备类型声明（model 字段） |
| `_rfb._tcp` | 5900 | VNC 远程桌面 |
| `_printer._tcp` | 631 | 网络打印机 |
| `_ipp._tcp` | 631 | IPP 打印协议 |
| `_net-assistant._udp` | 3283 | macOS 远程管理 |
| `_mi-connect._udp` | 56666 | 小米/Redmi 设备互联 |

---

## 依赖

| 包 | 版本 | 用途 |
|----|------|------|
| [github.com/miekg/dns](https://github.com/miekg/dns) | v1.1.62 | DNS 消息序列化/反序列化、记录类型解析 |

Go 标准库（`net`、`sync`、`strconv` 等）处理其余所有网络和并发逻辑。
