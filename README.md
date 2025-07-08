# 动能量化交易信号推送系统

## 📌 项目简介

本项目是一个基于 Go 语言开发的 **币安（Binance）合约市场动能量化信号筛选与推送系统**。系统定时扫描主流 USDT 交易对，结合 **成交量、EMA、StochRSI** 等多因子技术指标，自动筛选出具备买入或卖出信号的币种，并通过 **Telegram Bot** 实时推送，辅助量化交易者做出决策。

---

## 🚀 主要功能

- 实时获取币安合约市场所有 USDT 交易对的 24 小时成交量  
- 基于 **EMA25 / EMA50、StochRSI** 等多因子信号组合筛选  
- 高并发扫描，自动过滤低流动性及指定排除币种  
- **Telegram 机器人**实时推送交易信号至群组或频道  
- 支持 **MySQL** 存储 EMA 指标，便于后续分析与回测  

---

## 🧰 技术栈

- **Go 1.24+**
- **Binance API（go-binance/v2）**
- **MySQL**
- **Telegram Bot API**

---

## 📁 目录结构

```
 ├── main.go                 # 主程序入口，定时调度与核心逻辑
 ├── model/                 # 数据库连接与操作
 ├── types/                 # 项目结构体定义
 ├── utils/                 # 技术指标计算、数据抓取、推送等工具
 ├── telegram/              # Telegram 消息推送实现
 ├── cex_trade_mysql.sql    # MySQL 数据表结构
 ├── go.mod / go.sum        # Go 依赖管理
```

---

## 🛠 快速开始

### 1️⃣ 准备数据库

- 启动 MySQL 数据库  
- 导入 `cex_trade_mysql.sql` 文件，创建 `symbol_ema_1h` 表  

### 2️⃣ 配置参数

- 修改 `model/db.go` 中的数据库连接信息（默认：`root` / `Aa123456`）  
- 在 `main.go` 中填写：
  - Binance API Key / Secret  
  - Telegram Bot Token  
  - Telegram Chat ID  

### 3️⃣ 安装依赖

```bash
go mod tidy
```

## 4️⃣ 运行项目

```bash
go run main.go
```

## 🔁 系统流程

1. 启动后初始化 Binance API 客户端
2. 实时同步 24 小时成交量数据（REST + WebSocket）
3. 每分钟扫描所有 USDT 合约交易对，过滤流动性不足的币种
4. 并发拉取每个币种的 K 线数据，计算 EMA 与 StochRSI
5. 满足条件的币种生成买入/卖出信号，推送至 Telegram
6. 每小时将 EMA 数据持久化到 MySQL 数据库中

## 🧩 技术细节

- **高并发扫描**：结合 `WaitGroup` 和信号量，提升扫描效率
- **指标计算**：自定义实现 EMA、MACD、StochRSI 等主流指标
- **成交量缓存**：REST + WebSocket 双通道，保证成交量数据实时
- **推送策略**：信号分类推送，BTC/主流币高亮，避免刷屏干扰

------

## 🎯 应用场景

- 适用于量化交易员快速获取市场动能机会
- 可用于社群信号推送、策略验证与信号通知
- 配合回测系统进行交易策略研究与验证

------

## ⚠️ 免责声明

本项目仅用于 **学习与研究** 目的，涉及实际交易请务必自行评估风险。请勿将本项目用于任何非法用途。