# KuroHelper

KuroHelper 是持有 Discord Bot Token 的 Go 應用程式。除了原有的遊戲資訊功能，它現在也負責 Kuro 的 Discord 接入層：接收 Gateway 事件、判斷前綴或 mention、讀取最近頻道訊息、處理管理指令，並呼叫獨立的 AI Runtime。

## Kuro 架構分工

- 本專案：Discord Gateway/REST、頻道與使用者權限、slash commands、最終訊息發送。
- `kurohelper-service/kuro`：內部 WebSocket client、協定型別、觸發與最近上下文的純邏輯。
- `kurohelper-ai-runtime`：SillyTavern、OpenRouter、Persona、角色卡及長期記憶；不含 Bot Token。

## Kuro 環境變數

將 `.env.example` 複製為 `.env`，至少設定既有的 Bot/資料庫參數，以及：

```dotenv
KURO_RUNTIME_URL=ws://127.0.0.1:2334
KURO_RUNTIME_SECRET=與-ai-runtime-相同的長隨機字串
KURO_TRIGGER_PREFIX=小黑
KURO_CHANNEL_IDS=discord-channel-id
KURO_COMMAND_USER_IDS=admin-user-id-1,admin-user-id-2
KURO_RECENT_MESSAGE_LIMIT=15
KURO_RECENT_CONTEXT_CHARS=6000
KURO_REQUEST_TIMEOUT_SECONDS=180
```

`BOT_TOKEN` 只放在這個專案的 `.env`，不要放進 AI Runtime。

## 啟動

此 workspace 的 `go.work` 同時引用 `kurohelper` 與 `kurohelper-service`：

```powershell
go mod download
go run ./cmd
```

Discord Bot 需要啟用 Message Content Intent，並擁有 View Channel、Read Message History、Send Messages 與 Use Application Commands 權限。

## Kuro 操作

- 在允許的頻道使用 `小黑 ...` 或 mention Bot 觸發生成。
- 每次觸發時才向 Discord 抓取最近訊息，不在背景監聽時寫入記憶。
- Kuro 管理功能統一使用 `小黑 /英文指令` 文字格式；輸入 `小黑 /help` 可查看完整列表。
- `小黑 /newchat` 會設定該頻道的新上下文邊界；舊訊息仍存在 Discord，但不再注入新 prompt。
- `小黑 /status`、`小黑 /ai-stats [24h|7d|30d]`、記憶管理與備份復原指令都只允許 `KURO_COMMAND_USER_IDS` 中的使用者。`小黑 /memory-info <記憶ID>` 可查看單筆記憶的完整狀態，並透過記憶原有的 Discord 頻道與使用者訊息 ID 即時顯示來源訊息和跳轉連結；不保存原文，也不尋找 Kuro 回覆。`小黑 /memory-backups [頁碼]` 可列出整庫備份、`小黑 /memory-backup` 可立即備份，`小黑 /memory-rollback <備份ID> confirm` 可復原整個記憶庫；復原前會再自動建立安全備份。列表每頁顯示 5 條，省略頁碼時顯示第 1 頁。
- Kuro 不再註冊 `/小黑` 或 `/newchat` Discord Slash Command；其他既有遊戲資訊 Slash Command 不受影響。

新對話邊界保存在 PostgreSQL 的 `kuro_channel_states`；長期記憶管理則由 AI Runtime 的 memory-service 負責。

每次實際生成都會在 PostgreSQL 寫入一筆技術指標，包含成功狀態、各階段延遲、Token 與供應商費用；不保存 prompt、使用者訊息或模型回覆。單筆資料保留 30 天，每日彙總長期保留。
