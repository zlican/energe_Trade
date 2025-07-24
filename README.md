# 动能量化交易机器人

本项目是一个基于币安合约的动能量化交易机器人，具备多周期均线、动能、MACD、StochRSI 等多因子筛选与自动推送功能，适合用于数字货币量化策略的研究与实盘辅助。

## 功能简介

- **多周期均线与动能因子筛选**：
  - 支持 1H、15M、5M 等多周期 EMA（指数移动平均线）自动计算。
  - 支持 MACD（金叉/死叉预判）、StochRSI（动量超买超卖）等指标。
  - 结合多因子（均线、动能、MACD、StochRSI、成交量）进行买卖信号筛选。
- **高并发扫描与过滤**：
  - 自动拉取币安 USDT 合约市场所有币种，过滤成交量大于 3 亿 USDT 的币种。
  - 支持排除指定币种（如山寨币、流动性差币种等）。
  - 通过信号量和 WaitGroup 控制最大并发扫描数，提升效率。
- **自动推送与等待区管理**：
  - 将符合条件的币种信号自动推送到 Telegram 频道。
  - 设有等待区，避免重复推送同一信号。
  - 支持多 Bot Token，区分不同类型信号推送。
- **数据库存储与回测支持**：
  - 所有关键指标、信号、趋势等均存储于 MySQL，便于后续回测与分析。
  - 提供建表 SQL 脚本，方便初始化。
- **可配置代理与环境适配**：
  - 支持科学上网代理，适配国内网络环境。
  - 主要参数（API Key、Token、代理、并发数、成交量阈值等）均可灵活配置。
- **定时任务与多周期调度**：
  - 支持每 1 小时、15 分钟、5 分钟等多周期自动调度与扫描。
  - 自动对齐时间，保证数据同步。
- **日志与进度输出**：
  - 详细的进度日志，便于监控运行状态和调试。

## 目录结构

```
code/
├── main.go
│   └─ 主程序入口，负责初始化、定时调度、核心扫描、并发控制、主循环。
├── model/
│   └─ db.go
│       └─ 数据库初始化、连接管理、全局 DB 句柄。
├── types/
│   ├─ BETrend.go         # BTC/ETH 趋势结构体与相关类型
│   ├─ CoinIndicator.go   # 币种信号结构体（如买卖信号、指标等）
│   └─ VolumeCache.go     # 成交量缓存结构体与逻辑
├── utils/
│   ├─ 15MEMAToDB.go      # 15分钟 EMA 计算与存库
│   ├─ 1HEMA25WithDB.go   # 1小时 EMA 计算与存库
│   ├─ 5MEMAToDB.go       # 5分钟 EMA 计算与存库
│   ├─ AlignedTicker.go   # 时间对齐工具
│   ├─ calculateEMA.go    # EMA 指标计算
│   ├─ calculateMA.go     # MA 均线计算
│   ├─ calculateMACD.go   # MACD 指标计算与金叉死叉判断
│   ├─ calculateStochRSI.go # StochRSI 指标计算
│   ├─ coinInSlip.go      # 币种排除逻辑
│   ├─ get1MEMA.go        # 获取 1 分钟 EMA
│   ├─ get24HVolume.go    # 获取 24 小时成交量
│   ├─ getKlines.go       # 获取 K 线数据
│   ├─ getMainTrend.go    # 获取主趋势
│   ├─ pushTelegram.go    # Telegram 推送逻辑
│   ├─ volume_ws.go       # 成交量 WebSocket 相关
│   └─ waitEnerge.go      # 等待区管理逻辑
├── telegram/
│   └─ telegram.go        # Telegram 相关辅助逻辑
├── cex_trade_mysql.sql   # MySQL 数据库建表脚本
├── go.mod / go.sum       # Go 依赖管理文件
└── energe.exe            # 可执行文件（如有）
```

### 主要文件/目录说明

- **main.go**：
  - 程序主入口，负责初始化数据库、API 客户端、定时任务、主循环、并发扫描、信号推送等。
  - 包含核心的 `runScan`、`analyseSymbol` 等函数。
- **model/**：
  - 数据库连接与初始化，提供全局 DB 句柄。
- **types/**：
  - 定义所有核心结构体，如币种信号、趋势、成交量缓存等。
- **utils/**：
  - 所有指标计算、数据获取、推送、辅助工具函数。
  - 各周期 EMA、MACD、StochRSI 计算与存库，K 线拉取，成交量过滤，推送与等待区管理等。
- **telegram/**：
  - Telegram 相关辅助逻辑。
- **cex_trade_mysql.sql**：
  - MySQL 建表脚本，初始化数据库结构。
- **go.mod / go.sum**：
  - Go 依赖管理。

## 依赖环境

- Go 1.18 及以上
- MySQL 5.7 及以上
- [go-binance/v2](https://github.com/adshao/go-binance)
- 需自备币安 API Key/Secret
- Telegram Bot Token

## 安装与运行

1. **克隆项目**

   ```bash
   git clone https://github.com/yourname/yourrepo.git
   cd code
   ```

2. **安装依赖**

   ```bash
   go mod tidy
   ```

3. **配置数据库**

   - 创建数据库并导入 `cex_trade_mysql.sql`。
   - 修改 `model/db.go` 里的数据库连接信息。

4. **配置 API Key 和 Token**

   - 在 `main.go` 顶部填写你的币安 API Key、Secret、Telegram Bot Token。

5. **运行程序**
   ```bash
   go run main.go
   ```

## 核心逻辑说明

- **主循环**：程序启动后，定时（每 1 小时、每 15 分钟、每 5 分钟）自动拉取币安合约市场的 K 线数据，计算各类指标，并筛选出符合条件的币种。
- **多因子筛选**：结合 EMA、MACD、StochRSI、成交量等多因子，判断买入/卖出/观望信号。
- **并发处理**：通过信号量和 WaitGroup 控制最大并发数，提升扫描效率。
- **推送与等待区**：信号通过 Telegram Bot 推送到指定频道，并通过等待区机制避免重复推送。
- **数据库存储**：所有关键指标、信号、趋势等均存储于 MySQL，便于后续分析。

## 主要配置项

- `proxyURL`：科学上网代理地址，默认 `http://127.0.0.1:10809`
- `limitVolume`：最低成交量过滤，默认 2 亿 USDT
- `slipCoin`：排除币种列表
- `maxWorkers`：最大并发扫描数

## 常见问题

1. **无法连接币安/超时？**
   - 检查代理设置，确保 `proxyURL` 可用。
2. **推送失败？**
   - 检查 Telegram Bot Token 是否正确，Bot 是否已加入频道并有权限。
3. **数据库连接失败？**
   - 检查 MySQL 配置，确保账号、密码、端口正确，且已导入建表 SQL。

## 贡献与反馈

欢迎提交 Issue 或 PR 进行建议与改进！

---

如需英文版或更详细的技术细节，请告知！如果你有特殊的环境变量、配置文件需求，也可以补充说明。
