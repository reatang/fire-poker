# 配置兼容说明

Fire Poker：一个经典模式的智能体客户端，由 reatang 维护。

当前 Go 模块路径：`github.com/reatang/fire-poker`。

新安装使用 `.fire-poker` 数据目录、`FIRE_POKER_DATA_DIR` 环境变量和
`fire-poker-*` 浏览器配置键。已有安装继续识别 `.lemontea` 数据目录、
`LEMONTEA_DATA_DIR`、`.lemontea.json` Skill 元数据与旧浏览器配置键。
浏览器配置首次读取时复制至新键；旧值保留以供恢复。

历史聊天内容及已经锁定的系统提示词不会随品牌调整而改写。
