# Open WebUI Python Backend -> Go 重写计划

## 1. 目标

- 在 `backend/` 内重写 `open-webui/backend/open_webui` 的 Python 后端。
- 保持源码文件级别对齐，后续上游 Python 更新时，能够直接定位到对应的 Go 文件同步修改。
- 重写过程中不要求一次性切换，允许 Go 后端先作为“镜像骨架 + 逐文件替换 + 未完成部分回退到 Python”的过渡形态。
- 最终目标不是“接口大致可用”，而是以下四项同时成立：
- `open-webui/backend/open_webui/**/*.py` 每个源码文件都有一个规范对应的 `backend/open_webui/**/*.go` 文件。
- HTTP 路由、Socket 事件、数据库 schema、配置键、任务机制、存储行为与 Python 版本保持兼容。
- 前端不需要因为语言切换而调整 API 路径、事件名、鉴权方式和数据结构。
- 上游 Python 子模块升级后，可以用清单和 CI 立即发现哪些 Go 对应文件需要同步。

## 2. 当前基线

- 当前上游来源是 git submodule `open-webui`，提交为 `9bd84258d09eefe7bf975878fb0e31a5dadfe0f8`，标签显示为 `v0.8.12`。
- `open-webui/backend/open_webui` 共有 `221` 个 Python 源文件，总计约 `82,272` 行。
- 其中：

| 范围 | 文件数 | 行数 |
| --- | ---: | ---: |
| 根目录 | 7 | 8,321 |
| `internal` | 20 | 1,636 |
| `migrations` | 36 | 3,223 |
| `models` | 22 | 11,352 |
| `retrieval` | 58 | 12,530 |
| `routers` | 28 | 24,298 |
| `socket` | 2 | 1,190 |
| `storage` | 1 | 348 |
| `tools` | 2 | 2,332 |
| `utils` | 39 | 15,437 |
| `test` | 6 | 1,605 |

- 路由层已定义约 `404` 个 HTTP 路由。
- 体量最大的高风险文件：
- `utils/middleware.py`，`4738` 行。
- `config.py`，`4030` 行。
- `routers/retrieval.py`，`2676` 行。
- `main.py`，`2593` 行。
- `tools/builtin.py`，`2326` 行。
- `routers/ollama.py`，`1885` 行。
- `routers/channels.py`，`1822` 行。
- `utils/oauth.py`，`1695` 行。
- `models/chats.py`，`1556` 行。
- `routers/openai.py`，`1522` 行。
- `config.py` 中 `PersistentConfig(...)` 出现约 `343` 次，`env.py` 中环境变量读取约 `171` 处，这两个文件必须最早定型，否则后续所有模块都会反复返工。
- 当前 `backend/` 只有 `go.mod`，等于 Go 端还没有真正开始。

## 3. 文件级对齐规则

### 3.1 基本映射规则

- 默认规则：`foo.py -> foo.go`。
- 包初始化例外：`__init__.py -> init__.go`。
- Go 测试例外：`test_xxx.py -> test_xxx_test.go`。
- 迁移文件保留原始文件名基名：例如 `018012973d35_add_indexes.py -> 018012973d35_add_indexes.go`。

### 3.2 目录镜像规则

- Python 参考源码根目录：`open-webui/backend/open_webui/`
- Go 镜像根目录：`backend/open_webui/`
- 目录结构必须 1:1 镜像，例如：
- `open-webui/backend/open_webui/routers/openai.py`
- `backend/open_webui/routers/openai.go`

### 3.3 允许新增但不承载业务语义的辅助目录

- 允许新增 `backend/cmd/` 作为 Go 可执行入口。
- 允许新增 `backend/internal/platform/` 作为纯基础设施层，例如 HTTP 封装、数据库连接、测试工具、diff 工具。
- 允许新增 `backend/scripts/` 作为同步和校验脚本。
- 这些目录不能承载“某个 Python 源文件真正的业务逻辑”，业务逻辑仍然必须回到镜像文件里。

### 3.4 超大文件的受控拆分规则

- 默认坚持一对一单文件对应，不主动拆。
- 仅当 Go 实现因编译、测试、包循环依赖或可维护性必须拆分时，允许保留一个“规范主文件”再配少量同名前缀的伴随文件。
- 命名约定：
- 规范主文件：`middleware.go`
- 伴随文件：`middleware__stream.go`、`middleware__tools.go`
- 允许拆分的首批高风险文件仅限：
- `config.py`
- `main.py`
- `utils/middleware.py`
- `tools/builtin.py`
- `routers/ollama.py`
- `routers/retrieval.py`
- `routers/channels.py`
- `routers/chats.py`
- 即使拆分，仍要满足：
- 主文件名和 Python 文件名一一对应。
- Python 更新时，先检查主文件，再检查记录在映射清单里的伴随文件。
- 伴随文件只承载主文件中已经存在的语义切片，不能偷偷长出新的业务中心。

### 3.5 对齐元数据

- 需要新增一份机器可读清单，例如 `backend/SYNC_MAP.yaml` 或 `backend/SYNC_MAP.json`。
- 每条记录至少包含：
- `python_path`
- `go_path`
- `status`，建议取值：`stub`、`proxy`、`ported`、`parity_passed`
- `companions`
- `source_submodule_sha`
- `last_verified_at`
- 后续上游更新时，清单是第一道拦截器，不靠人工记忆。

## 4. 目标结构

```text
backend/
├── go.mod
├── cmd/
│   └── openwebui/
│       └── main.go
├── internal/
│   └── platform/
│       ├── dbx/
│       ├── httpx/
│       ├── proxy/
│       ├── syncmap/
│       └── testkit/
├── scripts/
│   ├── sync_inventory.sh
│   ├── compare_openapi.sh
│   └── compare_routes.sh
├── SYNC_MAP.yaml
└── open_webui/
    ├── init__.go
    ├── config.go
    ├── constants.go
    ├── env.go
    ├── functions.go
    ├── main.go
    ├── tasks.go
    ├── internal/
    ├── migrations/
    ├── models/
    ├── retrieval/
    ├── routers/
    ├── socket/
    ├── storage/
    ├── test/
    ├── tools/
    └── utils/
```

## 5. 推荐技术取舍

- HTTP 层优先使用 `net/http` + `chi`，原因是流式输出、反向代理、WebSocket、细粒度中间件控制会比偏“全家桶”的框架更稳。
- 数据层优先使用 `database/sql` + `bun` 或 `sqlx` 一类轻 ORM/半 ORM，而不是把业务全部交给重 ORM 自动推断。
- Redis 使用 `go-redis/v9`。
- JWT、OAuth、Session 走标准 Go 生态，不做自造轮子。
- 文件存储层按 `storage/provider.py` 的结构做 provider 接口，分别实现 local、S3、GCS、Azure。
- 迁移系统不直接套现成的“文件名受限”工具；建议自己做一个轻量 migration registry，这样能保留 Python 迁移文件名，避免破坏文件级对齐。
- Socket.IO 必须先做兼容性 spike，再决定最终库。这里不能先拍脑袋定库再重写，因为前端明确在走 `/ws/socket.io`。

## 6. 总体迁移策略

### 6.1 采用“镜像骨架 + Strangler”方式，不做大爆炸切换

- 第一步不是立刻把 Python 删掉，而是先让 Go 具备：
- 完整镜像目录树。
- 所有对应 Go 文件都存在并可编译。
- 未迁移路由自动回退到 Python 后端。
- 这样做的好处：
- 文件级映射关系从第一天就固定。
- 可以按文件逐步替换，不需要整批上线。
- 前端与部署路径不变，联调风险可控。

### 6.2 统一迁移循环

每个 Python 源文件都按同一个闭环迁移：

1. 在 `backend/open_webui/...` 生成对应 `.go` 文件。
2. 先写最小可编译骨架，并在 `SYNC_MAP` 中标记为 `stub`。
3. 若上层路径已接入 Go，但本文件语义未完成，则临时代理到 Python，标记为 `proxy`。
4. 把 Python 中的类型、常量、函数、类、SQL、路由、事件处理逐段迁入 Go。
5. 针对该文件补齐单测、契约测试或回归测试。
6. 与 Python 基线比对通过后，标记为 `parity_passed`。

### 6.3 上游 Python 始终是“参考真相”

- `open-webui/backend/open_webui` 在整个重写期间都保留。
- Go 端不是重新设计一个新后端，而是翻译、约束、稳定化这个 Python 后端。
- 除非明确决定背离上游，否则不要擅自改 API、改路径、改配置名、改表结构、改事件名。

## 7. 分阶段实施计划

## Phase 0：冻结基线与自动化清单

目标：

- 把当前 Python 后端的文件、路由、配置键、环境变量、迁移版本、模型表结构全部提取成可比对清单。

动作：

- 固定子模块版本为当前 SHA。
- 生成源码文件清单。
- 生成路由清单，至少包含方法、路径、处理函数名。
- 生成配置键清单，尤其是 `config.py` 的持久化配置项。
- 生成环境变量清单，来自 `env.py`。
- 生成数据库迁移清单，包含 `internal/migrations` 和 `migrations/versions` 两套。
- 生成 OpenAPI 基线。

产物：

- `backend/SYNC_MAP.yaml`
- `backend/scripts/sync_inventory.sh`
- `backend/scripts/compare_routes.sh`
- `backend/scripts/compare_openapi.sh`

验收：

- 任意新增、删除、重命名的 Python 源文件都能在 CI 里直接报出来。

## Phase 1：建立完整 Go 镜像骨架并让它可启动

目标：

- 在不实现业务的前提下，先把 `221` 个镜像文件全部创建出来，并让 Go 服务可启动。

优先文件：

- 根目录：`init__.go`、`constants.go`、`env.go`、`main.go`
- 基础设施：`internal/db.go`、`internal/wrappers.go`
- 基础工具：`utils/logger.go`、`utils/response.go`、`utils/security_headers.go`、`utils/rate_limit.go`、`utils/redis.go`

动作：

- 建立 `backend/open_webui` 全量目录树。
- 批量生成所有对应 `.go` 文件，至少包含正确 package 声明。
- 建立 `backend/cmd/openwebui/main.go`，启动 Go HTTP 服务。
- 在 `main.go` 中先实现统一反向代理，把未接管的路径转发给 Python 服务。
- 保持静态文件、前端构建目录、数据目录、数据库、Redis 指向与 Python 一致。

验收：

- `go test ./...` 至少能跑到基础骨架级别。
- Go 服务可以启动。
- 所有未迁移请求能透传给 Python。
- `SYNC_MAP` 中每个 Python 源文件都有记录。

## Phase 2：数据层与迁移系统

目标：

- 先打稳数据库底座，避免后面每迁一个 router 都反复返修模型层。

必须先迁的文件：

- `config.py`
- `internal/db.py`
- `internal/migrations/*.py`
- `migrations/env.py`
- `migrations/util.py`
- `migrations/versions/*.py`

随后迁移的模型文件：

- `models/users.py`
- `models/auths.py`
- `models/groups.py`
- `models/access_grants.py`
- `models/oauth_sessions.py`
- `models/files.py`
- `models/folders.py`
- `models/models.py`
- `models/functions.py`
- `models/tools.py`
- `models/skills.py`
- `models/prompts.py`
- `models/memories.py`
- `models/notes.py`
- `models/feedbacks.py`
- `models/tags.py`
- `models/chats.py`
- `models/messages.py`
- `models/chat_messages.py`
- `models/channels.py`
- `models/knowledge.py`
- `models/prompt_history.py`

关键要求：

- Go 端迁移文件名与 Python 保持对应，不重新编号。
- 先实现“执行顺序与 schema 一致”，再考虑是否做更优雅的 DSL。
- Python 中 Pydantic 模型、SQLAlchemy 模型、查询帮助函数，尽量仍然留在同一个对应 Go 文件中。
- 对于 `config.py`，先实现读写兼容和持久化兼容，再考虑内部重构。

验收：

- 空库下，Go 端能独立完成 schema 初始化。
- 已有 Python 库迁移到最新版本后，Go 端能无损读取。
- 核心表的 CRUD 与 Python 输出一致。

## Phase 3：身份、鉴权、配置、组织结构

目标：

- 先打通“所有人都会依赖”的公共能力：登录、用户、管理员、分组、权限、SCIM、配置。

核心文件：

- `utils/auth.py`
- `utils/oauth.py`
- `utils/access_control/__init__.py`
- `utils/access_control/files.py`
- `utils/audit.py`
- `utils/groups.py`
- `utils/headers.py`
- `utils/validate.py`
- `routers/auths.py`
- `routers/users.py`
- `routers/groups.py`
- `routers/scim.py`
- `routers/configs.py`
- `routers/analytics.py`
- `routers/utils.py`

顺序建议：

1. `models/users.go`、`models/auths.go`、`models/groups.go`、`models/access_grants.go`
2. `utils/auth.go`、`utils/oauth.go`
3. `routers/auths.go`
4. `routers/users.go`
5. `routers/groups.go`
6. `routers/configs.go`
7. `routers/scim.go`、`routers/analytics.go`

验收：

- 登录、注册、token、API key、管理员权限、组权限、SCIM 都能走 Go。
- 对应路由不再回退到 Python。

## Phase 4：基础资源 CRUD、存储、模型元数据

目标：

- 把“结构化 CRUD 且风险相对可控”的模块先搬走。

核心文件：

- `storage/provider.py`
- `utils/files.py`
- `utils/filter.py`
- `utils/plugin.py`
- `utils/pdf_generator.py`
- `utils/webhook.py`
- `routers/files.py`
- `routers/folders.py`
- `routers/memories.py`
- `routers/notes.py`
- `routers/prompts.py`
- `routers/functions.py`
- `routers/tools.py`
- `routers/skills.py`
- `routers/evaluations.py`
- `routers/models.py`

策略：

- `storage/provider.go` 先定义统一接口，再逐个 provider 实现。
- 先完成 local storage，再做 S3、GCS、Azure。
- `models.py` 和 `routers/models.py` 的“模型元数据”能力先迁，但 `openai.py` / `ollama.py` 里的真正模型代理能力放到后面单独做。

验收：

- 这些 CRUD 类接口全部走 Go。
- 文件上传、下载、存储后端切换与 Python 行为一致。

## Phase 5：检索、知识库、向量库、Web 搜索

目标：

- 这部分文件多、provider 多、外部依赖多，必须整块作为一个独立阶段推进。

核心文件：

- `retrieval/loaders/*.py`
- `retrieval/models/*.py`
- `retrieval/vector/*.py`
- `retrieval/vector/dbs/*.py`
- `retrieval/web/*.py`
- `retrieval/utils.py`
- `models/knowledge.py`
- `routers/retrieval.py`
- `routers/knowledge.py`

策略：

- 先建立 `retrieval/vector` 的抽象接口，再补 `factory.go`。
- 首批优先实现最常用 provider：
- `chroma`
- `pgvector`
- `qdrant`
- `milvus`
- Web 搜索 provider 先做统一接口，再逐个迁移。
- 所有长尾 provider 对应的 `.go` 文件在本阶段一开始就生成，即使先留空壳或代理逻辑，也要把对齐关系固定住。

验收：

- 文档入库、切片、向量化、召回、重排、Web 检索都能在 Go 端完成。
- Python 的 retrieval 路由可完全下线。

## Phase 6：模型代理、推理链路、聊天中间层

目标：

- 先迁对上游模型服务的代理层，再迁聊天编排层。

核心文件：

- `routers/openai.py`
- `routers/ollama.py`
- `functions.py`
- `utils/models.py`
- `utils/payload.py`
- `utils/anthropic.py`
- `utils/chat.py`
- `utils/actions.py`
- `utils/tools.py`
- `utils/code_interpreter.py`
- `utils/images/comfyui.py`

关键点：

- `routers/openai.py` 和 `routers/ollama.py` 是整个聊天链路的外部模型出口，先迁它们，后面的聊天编排才能真正落地。
- `functions.py` 中的 function module 装载逻辑要尽量保持同样的目录与调用关系。
- `utils/tools.py`、`utils/actions.py`、`utils/payload.py` 要优先稳定，因为 `utils/middleware.py` 后面会大量依赖它们。

验收：

- 非聊天场景下的模型列表、completion、chat completion、embedding、模型管理能力都能由 Go 接管。

## Phase 7：聊天主流程、媒体、频道、内建工具

目标：

- 处理体量最大、依赖最深、最容易引入回归的核心业务。

核心文件：

- `utils/middleware.py`
- `models/chats.py`
- `models/messages.py`
- `models/chat_messages.py`
- `models/channels.py`
- `routers/chats.py`
- `routers/channels.py`
- `routers/audio.py`
- `routers/images.py`
- `routers/pipelines.py`
- `tools/builtin.py`

策略：

- `utils/middleware.py` 不要一上来就追求重构，先按 Python 的函数顺序搬，确保流式返回、工具调用、文件上下文、RAG 注入、事件流、后台任务回调都可工作。
- `tools/builtin.py` 与聊天工具调用链强耦合，要和 `utils/middleware.go` 联动测试。
- `routers/chats.py` 与 `models/chats.go`、`models/messages.go` 应作为一个组合包推进。
- `routers/channels.py` 与 `socket/main.go` 后续也会强耦合，但这里先完成 HTTP 侧的数据与事件入口。

验收：

- 从前端发起一次完整聊天请求，包含鉴权、模型路由、工具调用、文件上下文、RAG、流式输出、消息落库，全部可由 Go 完成。

## Phase 8：实时通信、任务系统、终端与 Socket

目标：

- 把 Python 时代的异步实时能力彻底搬完。

核心文件：

- `socket/main.py`
- `socket/utils.py`
- `tasks.py`
- `utils/task.py`
- `utils/mcp/client.py`
- `routers/tasks.py`
- `routers/terminals.py`

关键点：

- 前端当前明确依赖 Socket.IO，路径为 `/ws/socket.io`，并且带 Yjs 协作逻辑，这块必须做兼容性回放测试。
- `tasks.py` 使用 Redis 做任务控制，Go 端也要保留同样的 key 语义与停止机制。
- `routers/terminals.py`、`utils/mcp/client.py` 涉及外部长连接和工具调用，需要单独做超时、取消和权限验证。

验收：

- Socket 事件、房间管理、文档协作、任务停止、终端事件都稳定运行。
- Python 的 `/ws` 可以下线。

## Phase 9：测试、灰度、切换、清理

目标：

- 不留“代码看起来写完了，但不知道是否等价”的灰区。

动作：

- 将 `open_webui/test/**/*.py` 映射为 Go 测试并补齐缺失场景。
- 增加契约测试：
- OpenAPI 对比
- 路由清单对比
- 鉴权行为对比
- 流式响应分块对比
- Socket 事件回放对比
- DB schema 与关键 SQL 结果对比
- 增加 shadow traffic 或双写校验。
- 分阶段关闭 Python 回退路径。
- 当某个路由组完全通过 parity 后，再把该组回退逻辑删除。

验收：

- 默认流量全部走 Go。
- Python 后端仅保留为参考源码，不再参与线上转发。

## 8. 高风险问题与处理方式

### 8.1 `config.py` 和 `env.py` 不是普通配置文件

- 这两个文件几乎是全局装配中心。
- 如果早期没有先定义好 Go 侧的持久化配置模型、环境变量解析模型、默认值合并规则，后面所有 router 都会来回重写。
- 处理方式：在 Phase 2 完成前，不开始大规模迁移业务 router。

### 8.2 数据库迁移有两套体系

- Python 同时存在 `internal/migrations` 和 Alembic `migrations/versions`。
- 这意味着 Go 不能简单只认一套版本表。
- 处理方式：Go 先做兼容执行器，按 Python 当前真实执行顺序重放。

### 8.3 `utils/middleware.py` 是核心中的核心

- 它包含聊天编排、工具调用、文件上下文、RAG、流式事件、后台任务整合。
- 这是整个项目最不应该“重构式重写”的文件。
- 处理方式：先翻译，再验证，再局部提炼；不要先做架构洁癖。

### 8.4 Socket.IO + Yjs 是协议风险点

- 不是普通 WebSocket 替换。
- 处理方式：尽早做兼容性 spike；如果两周内前端联调不过，先保留 Python socket sidecar，不阻塞其余模块切换。

### 8.5 Retrieval/provider 矩阵很大

- `retrieval` 目录有 `58` 个文件，向量库和 Web 搜索 provider 很多。
- 处理方式：先把接口与骨架铺满，再按优先级补 provider，不要等到所有 provider 都要实现时才开始建目录和清单。

## 9. 上游 Python 更新时的同步流程

1. 更新 `open-webui` 子模块。
2. 运行清单脚本，检查：
- 是否新增、删除、重命名了 Python 源文件。
- 是否新增了路由。
- 是否新增了环境变量或持久化配置项。
- 是否新增了数据库迁移。
3. 对于每一个变化的 Python 文件，直接打开对应 Go 文件。
4. 若该文件存在伴随文件，同时检查 `SYNC_MAP` 中的 companions。
5. 先同步签名、常量、数据结构，再同步行为。
6. 运行目标文件所属模块的契约测试。
7. 通过后更新 `SYNC_MAP` 中的 `source_submodule_sha` 与状态。

这个流程的关键不是“能更新”，而是“任何未同步都能被自动发现”。

## 10. 非源码文件的处理原则

- 以下文件不是这次“文件级源码对齐”的主体，但需要保留为参考：
- `open-webui/backend/dev.sh`
- `open-webui/backend/start.sh`
- `open-webui/backend/start_windows.bat`
- `open-webui/backend/requirements.txt`
- `open-webui/backend/requirements-min.txt`
- `open-webui/backend/open_webui/alembic.ini`
- `open-webui/backend/open_webui/migrations/script.py.mako`
- `open-webui/backend/open_webui/static/**`
- `open-webui/backend/open_webui/data/**`
- 这些文件可以在 Go 侧转换成启动脚本、Make/Task 命令、嵌入资源或部署配置，但不需要强行 1:1 改成 Go 文件。

## 11. 完整文件映射清单

## root
- `open-webui/backend/open_webui/__init__.py` -> `backend/open_webui/init__.go`
- `open-webui/backend/open_webui/config.py` -> `backend/open_webui/config.go`
- `open-webui/backend/open_webui/constants.py` -> `backend/open_webui/constants.go`
- `open-webui/backend/open_webui/env.py` -> `backend/open_webui/env.go`
- `open-webui/backend/open_webui/functions.py` -> `backend/open_webui/functions.go`
- `open-webui/backend/open_webui/main.py` -> `backend/open_webui/main.go`
- `open-webui/backend/open_webui/tasks.py` -> `backend/open_webui/tasks.go`

## internal
- `open-webui/backend/open_webui/internal/db.py` -> `backend/open_webui/internal/db.go`
- `open-webui/backend/open_webui/internal/migrations/001_initial_schema.py` -> `backend/open_webui/internal/migrations/001_initial_schema.go`
- `open-webui/backend/open_webui/internal/migrations/002_add_local_sharing.py` -> `backend/open_webui/internal/migrations/002_add_local_sharing.go`
- `open-webui/backend/open_webui/internal/migrations/003_add_auth_api_key.py` -> `backend/open_webui/internal/migrations/003_add_auth_api_key.go`
- `open-webui/backend/open_webui/internal/migrations/004_add_archived.py` -> `backend/open_webui/internal/migrations/004_add_archived.go`
- `open-webui/backend/open_webui/internal/migrations/005_add_updated_at.py` -> `backend/open_webui/internal/migrations/005_add_updated_at.go`
- `open-webui/backend/open_webui/internal/migrations/006_migrate_timestamps_and_charfields.py` -> `backend/open_webui/internal/migrations/006_migrate_timestamps_and_charfields.go`
- `open-webui/backend/open_webui/internal/migrations/007_add_user_last_active_at.py` -> `backend/open_webui/internal/migrations/007_add_user_last_active_at.go`
- `open-webui/backend/open_webui/internal/migrations/008_add_memory.py` -> `backend/open_webui/internal/migrations/008_add_memory.go`
- `open-webui/backend/open_webui/internal/migrations/009_add_models.py` -> `backend/open_webui/internal/migrations/009_add_models.go`
- `open-webui/backend/open_webui/internal/migrations/010_migrate_modelfiles_to_models.py` -> `backend/open_webui/internal/migrations/010_migrate_modelfiles_to_models.go`
- `open-webui/backend/open_webui/internal/migrations/011_add_user_settings.py` -> `backend/open_webui/internal/migrations/011_add_user_settings.go`
- `open-webui/backend/open_webui/internal/migrations/012_add_tools.py` -> `backend/open_webui/internal/migrations/012_add_tools.go`
- `open-webui/backend/open_webui/internal/migrations/013_add_user_info.py` -> `backend/open_webui/internal/migrations/013_add_user_info.go`
- `open-webui/backend/open_webui/internal/migrations/014_add_files.py` -> `backend/open_webui/internal/migrations/014_add_files.go`
- `open-webui/backend/open_webui/internal/migrations/015_add_functions.py` -> `backend/open_webui/internal/migrations/015_add_functions.go`
- `open-webui/backend/open_webui/internal/migrations/016_add_valves_and_is_active.py` -> `backend/open_webui/internal/migrations/016_add_valves_and_is_active.go`
- `open-webui/backend/open_webui/internal/migrations/017_add_user_oauth_sub.py` -> `backend/open_webui/internal/migrations/017_add_user_oauth_sub.go`
- `open-webui/backend/open_webui/internal/migrations/018_add_function_is_global.py` -> `backend/open_webui/internal/migrations/018_add_function_is_global.go`
- `open-webui/backend/open_webui/internal/wrappers.py` -> `backend/open_webui/internal/wrappers.go`

## migrations
- `open-webui/backend/open_webui/migrations/env.py` -> `backend/open_webui/migrations/env.go`
- `open-webui/backend/open_webui/migrations/util.py` -> `backend/open_webui/migrations/util.go`
- `open-webui/backend/open_webui/migrations/versions/018012973d35_add_indexes.py` -> `backend/open_webui/migrations/versions/018012973d35_add_indexes.go`
- `open-webui/backend/open_webui/migrations/versions/1af9b942657b_migrate_tags.py` -> `backend/open_webui/migrations/versions/1af9b942657b_migrate_tags.go`
- `open-webui/backend/open_webui/migrations/versions/242a2047eae0_update_chat_table.py` -> `backend/open_webui/migrations/versions/242a2047eae0_update_chat_table.go`
- `open-webui/backend/open_webui/migrations/versions/2f1211949ecc_update_message_and_channel_member_table.py` -> `backend/open_webui/migrations/versions/2f1211949ecc_update_message_and_channel_member_table.go`
- `open-webui/backend/open_webui/migrations/versions/374d2f66af06_add_prompt_history_table.py` -> `backend/open_webui/migrations/versions/374d2f66af06_add_prompt_history_table.go`
- `open-webui/backend/open_webui/migrations/versions/3781e22d8b01_update_message_table.py` -> `backend/open_webui/migrations/versions/3781e22d8b01_update_message_table.go`
- `open-webui/backend/open_webui/migrations/versions/37f288994c47_add_group_member_table.py` -> `backend/open_webui/migrations/versions/37f288994c47_add_group_member_table.go`
- `open-webui/backend/open_webui/migrations/versions/38d63c18f30f_add_oauth_session_table.py` -> `backend/open_webui/migrations/versions/38d63c18f30f_add_oauth_session_table.go`
- `open-webui/backend/open_webui/migrations/versions/3ab32c4b8f59_update_tags.py` -> `backend/open_webui/migrations/versions/3ab32c4b8f59_update_tags.go`
- `open-webui/backend/open_webui/migrations/versions/3af16a1c9fb6_update_user_table.py` -> `backend/open_webui/migrations/versions/3af16a1c9fb6_update_user_table.go`
- `open-webui/backend/open_webui/migrations/versions/3e0e00844bb0_add_knowledge_file_table.py` -> `backend/open_webui/migrations/versions/3e0e00844bb0_add_knowledge_file_table.go`
- `open-webui/backend/open_webui/migrations/versions/4ace53fd72c8_update_folder_table_datetime.py` -> `backend/open_webui/migrations/versions/4ace53fd72c8_update_folder_table_datetime.go`
- `open-webui/backend/open_webui/migrations/versions/57c599a3cb57_add_channel_table.py` -> `backend/open_webui/migrations/versions/57c599a3cb57_add_channel_table.go`
- `open-webui/backend/open_webui/migrations/versions/6283dc0e4d8d_add_channel_file_table.py` -> `backend/open_webui/migrations/versions/6283dc0e4d8d_add_channel_file_table.go`
- `open-webui/backend/open_webui/migrations/versions/6a39f3d8e55c_add_knowledge_table.py` -> `backend/open_webui/migrations/versions/6a39f3d8e55c_add_knowledge_table.go`
- `open-webui/backend/open_webui/migrations/versions/7826ab40b532_update_file_table.py` -> `backend/open_webui/migrations/versions/7826ab40b532_update_file_table.go`
- `open-webui/backend/open_webui/migrations/versions/7e5b5dc7342b_init.py` -> `backend/open_webui/migrations/versions/7e5b5dc7342b_init.go`
- `open-webui/backend/open_webui/migrations/versions/81cc2ce44d79_update_channel_file_and_knowledge_table.py` -> `backend/open_webui/migrations/versions/81cc2ce44d79_update_channel_file_and_knowledge_table.go`
- `open-webui/backend/open_webui/migrations/versions/8452d01d26d7_add_chat_message_table.py` -> `backend/open_webui/migrations/versions/8452d01d26d7_add_chat_message_table.go`
- `open-webui/backend/open_webui/migrations/versions/90ef40d4714e_update_channel_and_channel_members_table.py` -> `backend/open_webui/migrations/versions/90ef40d4714e_update_channel_and_channel_members_table.go`
- `open-webui/backend/open_webui/migrations/versions/922e7a387820_add_group_table.py` -> `backend/open_webui/migrations/versions/922e7a387820_add_group_table.go`
- `open-webui/backend/open_webui/migrations/versions/9f0c9cd09105_add_note_table.py` -> `backend/open_webui/migrations/versions/9f0c9cd09105_add_note_table.go`
- `open-webui/backend/open_webui/migrations/versions/a1b2c3d4e5f6_add_skill_table.py` -> `backend/open_webui/migrations/versions/a1b2c3d4e5f6_add_skill_table.go`
- `open-webui/backend/open_webui/migrations/versions/a5c220713937_add_reply_to_id_column_to_message.py` -> `backend/open_webui/migrations/versions/a5c220713937_add_reply_to_id_column_to_message.go`
- `open-webui/backend/open_webui/migrations/versions/af906e964978_add_feedback_table.py` -> `backend/open_webui/migrations/versions/af906e964978_add_feedback_table.go`
- `open-webui/backend/open_webui/migrations/versions/b10670c03dd5_update_user_table.py` -> `backend/open_webui/migrations/versions/b10670c03dd5_update_user_table.go`
- `open-webui/backend/open_webui/migrations/versions/b2c3d4e5f6a7_add_scim_column_to_user_table.py` -> `backend/open_webui/migrations/versions/b2c3d4e5f6a7_add_scim_column_to_user_table.go`
- `open-webui/backend/open_webui/migrations/versions/c0fbf31ca0db_update_file_table.py` -> `backend/open_webui/migrations/versions/c0fbf31ca0db_update_file_table.go`
- `open-webui/backend/open_webui/migrations/versions/c29facfe716b_update_file_table_path.py` -> `backend/open_webui/migrations/versions/c29facfe716b_update_file_table_path.go`
- `open-webui/backend/open_webui/migrations/versions/c440947495f3_add_chat_file_table.py` -> `backend/open_webui/migrations/versions/c440947495f3_add_chat_file_table.go`
- `open-webui/backend/open_webui/migrations/versions/c69f45358db4_add_folder_table.py` -> `backend/open_webui/migrations/versions/c69f45358db4_add_folder_table.go`
- `open-webui/backend/open_webui/migrations/versions/ca81bd47c050_add_config_table.py` -> `backend/open_webui/migrations/versions/ca81bd47c050_add_config_table.go`
- `open-webui/backend/open_webui/migrations/versions/d31026856c01_update_folder_table_data.py` -> `backend/open_webui/migrations/versions/d31026856c01_update_folder_table_data.go`
- `open-webui/backend/open_webui/migrations/versions/f1e2d3c4b5a6_add_access_grant_table.py` -> `backend/open_webui/migrations/versions/f1e2d3c4b5a6_add_access_grant_table.go`

## models
- `open-webui/backend/open_webui/models/access_grants.py` -> `backend/open_webui/models/access_grants.go`
- `open-webui/backend/open_webui/models/auths.py` -> `backend/open_webui/models/auths.go`
- `open-webui/backend/open_webui/models/channels.py` -> `backend/open_webui/models/channels.go`
- `open-webui/backend/open_webui/models/chat_messages.py` -> `backend/open_webui/models/chat_messages.go`
- `open-webui/backend/open_webui/models/chats.py` -> `backend/open_webui/models/chats.go`
- `open-webui/backend/open_webui/models/feedbacks.py` -> `backend/open_webui/models/feedbacks.go`
- `open-webui/backend/open_webui/models/files.py` -> `backend/open_webui/models/files.go`
- `open-webui/backend/open_webui/models/folders.py` -> `backend/open_webui/models/folders.go`
- `open-webui/backend/open_webui/models/functions.py` -> `backend/open_webui/models/functions.go`
- `open-webui/backend/open_webui/models/groups.py` -> `backend/open_webui/models/groups.go`
- `open-webui/backend/open_webui/models/knowledge.py` -> `backend/open_webui/models/knowledge.go`
- `open-webui/backend/open_webui/models/memories.py` -> `backend/open_webui/models/memories.go`
- `open-webui/backend/open_webui/models/messages.py` -> `backend/open_webui/models/messages.go`
- `open-webui/backend/open_webui/models/models.py` -> `backend/open_webui/models/models.go`
- `open-webui/backend/open_webui/models/notes.py` -> `backend/open_webui/models/notes.go`
- `open-webui/backend/open_webui/models/oauth_sessions.py` -> `backend/open_webui/models/oauth_sessions.go`
- `open-webui/backend/open_webui/models/prompt_history.py` -> `backend/open_webui/models/prompt_history.go`
- `open-webui/backend/open_webui/models/prompts.py` -> `backend/open_webui/models/prompts.go`
- `open-webui/backend/open_webui/models/skills.py` -> `backend/open_webui/models/skills.go`
- `open-webui/backend/open_webui/models/tags.py` -> `backend/open_webui/models/tags.go`
- `open-webui/backend/open_webui/models/tools.py` -> `backend/open_webui/models/tools.go`
- `open-webui/backend/open_webui/models/users.py` -> `backend/open_webui/models/users.go`

## retrieval
- `open-webui/backend/open_webui/retrieval/loaders/datalab_marker.py` -> `backend/open_webui/retrieval/loaders/datalab_marker.go`
- `open-webui/backend/open_webui/retrieval/loaders/external_document.py` -> `backend/open_webui/retrieval/loaders/external_document.go`
- `open-webui/backend/open_webui/retrieval/loaders/external_web.py` -> `backend/open_webui/retrieval/loaders/external_web.go`
- `open-webui/backend/open_webui/retrieval/loaders/main.py` -> `backend/open_webui/retrieval/loaders/main.go`
- `open-webui/backend/open_webui/retrieval/loaders/mineru.py` -> `backend/open_webui/retrieval/loaders/mineru.go`
- `open-webui/backend/open_webui/retrieval/loaders/mistral.py` -> `backend/open_webui/retrieval/loaders/mistral.go`
- `open-webui/backend/open_webui/retrieval/loaders/tavily.py` -> `backend/open_webui/retrieval/loaders/tavily.go`
- `open-webui/backend/open_webui/retrieval/loaders/youtube.py` -> `backend/open_webui/retrieval/loaders/youtube.go`
- `open-webui/backend/open_webui/retrieval/models/base_reranker.py` -> `backend/open_webui/retrieval/models/base_reranker.go`
- `open-webui/backend/open_webui/retrieval/models/colbert.py` -> `backend/open_webui/retrieval/models/colbert.go`
- `open-webui/backend/open_webui/retrieval/models/external.py` -> `backend/open_webui/retrieval/models/external.go`
- `open-webui/backend/open_webui/retrieval/utils.py` -> `backend/open_webui/retrieval/utils.go`
- `open-webui/backend/open_webui/retrieval/vector/dbs/chroma.py` -> `backend/open_webui/retrieval/vector/dbs/chroma.go`
- `open-webui/backend/open_webui/retrieval/vector/dbs/elasticsearch.py` -> `backend/open_webui/retrieval/vector/dbs/elasticsearch.go`
- `open-webui/backend/open_webui/retrieval/vector/dbs/mariadb_vector.py` -> `backend/open_webui/retrieval/vector/dbs/mariadb_vector.go`
- `open-webui/backend/open_webui/retrieval/vector/dbs/milvus.py` -> `backend/open_webui/retrieval/vector/dbs/milvus.go`
- `open-webui/backend/open_webui/retrieval/vector/dbs/milvus_multitenancy.py` -> `backend/open_webui/retrieval/vector/dbs/milvus_multitenancy.go`
- `open-webui/backend/open_webui/retrieval/vector/dbs/opengauss.py` -> `backend/open_webui/retrieval/vector/dbs/opengauss.go`
- `open-webui/backend/open_webui/retrieval/vector/dbs/opensearch.py` -> `backend/open_webui/retrieval/vector/dbs/opensearch.go`
- `open-webui/backend/open_webui/retrieval/vector/dbs/oracle23ai.py` -> `backend/open_webui/retrieval/vector/dbs/oracle23ai.go`
- `open-webui/backend/open_webui/retrieval/vector/dbs/pgvector.py` -> `backend/open_webui/retrieval/vector/dbs/pgvector.go`
- `open-webui/backend/open_webui/retrieval/vector/dbs/pinecone.py` -> `backend/open_webui/retrieval/vector/dbs/pinecone.go`
- `open-webui/backend/open_webui/retrieval/vector/dbs/qdrant.py` -> `backend/open_webui/retrieval/vector/dbs/qdrant.go`
- `open-webui/backend/open_webui/retrieval/vector/dbs/qdrant_multitenancy.py` -> `backend/open_webui/retrieval/vector/dbs/qdrant_multitenancy.go`
- `open-webui/backend/open_webui/retrieval/vector/dbs/s3vector.py` -> `backend/open_webui/retrieval/vector/dbs/s3vector.go`
- `open-webui/backend/open_webui/retrieval/vector/dbs/weaviate.py` -> `backend/open_webui/retrieval/vector/dbs/weaviate.go`
- `open-webui/backend/open_webui/retrieval/vector/factory.py` -> `backend/open_webui/retrieval/vector/factory.go`
- `open-webui/backend/open_webui/retrieval/vector/main.py` -> `backend/open_webui/retrieval/vector/main.go`
- `open-webui/backend/open_webui/retrieval/vector/type.py` -> `backend/open_webui/retrieval/vector/type.go`
- `open-webui/backend/open_webui/retrieval/vector/utils.py` -> `backend/open_webui/retrieval/vector/utils.go`
- `open-webui/backend/open_webui/retrieval/web/azure.py` -> `backend/open_webui/retrieval/web/azure.go`
- `open-webui/backend/open_webui/retrieval/web/bing.py` -> `backend/open_webui/retrieval/web/bing.go`
- `open-webui/backend/open_webui/retrieval/web/bocha.py` -> `backend/open_webui/retrieval/web/bocha.go`
- `open-webui/backend/open_webui/retrieval/web/brave.py` -> `backend/open_webui/retrieval/web/brave.go`
- `open-webui/backend/open_webui/retrieval/web/duckduckgo.py` -> `backend/open_webui/retrieval/web/duckduckgo.go`
- `open-webui/backend/open_webui/retrieval/web/exa.py` -> `backend/open_webui/retrieval/web/exa.go`
- `open-webui/backend/open_webui/retrieval/web/external.py` -> `backend/open_webui/retrieval/web/external.go`
- `open-webui/backend/open_webui/retrieval/web/firecrawl.py` -> `backend/open_webui/retrieval/web/firecrawl.go`
- `open-webui/backend/open_webui/retrieval/web/google_pse.py` -> `backend/open_webui/retrieval/web/google_pse.go`
- `open-webui/backend/open_webui/retrieval/web/jina_search.py` -> `backend/open_webui/retrieval/web/jina_search.go`
- `open-webui/backend/open_webui/retrieval/web/kagi.py` -> `backend/open_webui/retrieval/web/kagi.go`
- `open-webui/backend/open_webui/retrieval/web/main.py` -> `backend/open_webui/retrieval/web/main.go`
- `open-webui/backend/open_webui/retrieval/web/mojeek.py` -> `backend/open_webui/retrieval/web/mojeek.go`
- `open-webui/backend/open_webui/retrieval/web/ollama.py` -> `backend/open_webui/retrieval/web/ollama.go`
- `open-webui/backend/open_webui/retrieval/web/perplexity.py` -> `backend/open_webui/retrieval/web/perplexity.go`
- `open-webui/backend/open_webui/retrieval/web/perplexity_search.py` -> `backend/open_webui/retrieval/web/perplexity_search.go`
- `open-webui/backend/open_webui/retrieval/web/searchapi.py` -> `backend/open_webui/retrieval/web/searchapi.go`
- `open-webui/backend/open_webui/retrieval/web/searxng.py` -> `backend/open_webui/retrieval/web/searxng.go`
- `open-webui/backend/open_webui/retrieval/web/serpapi.py` -> `backend/open_webui/retrieval/web/serpapi.go`
- `open-webui/backend/open_webui/retrieval/web/serper.py` -> `backend/open_webui/retrieval/web/serper.go`
- `open-webui/backend/open_webui/retrieval/web/serply.py` -> `backend/open_webui/retrieval/web/serply.go`
- `open-webui/backend/open_webui/retrieval/web/serpstack.py` -> `backend/open_webui/retrieval/web/serpstack.go`
- `open-webui/backend/open_webui/retrieval/web/sougou.py` -> `backend/open_webui/retrieval/web/sougou.go`
- `open-webui/backend/open_webui/retrieval/web/tavily.py` -> `backend/open_webui/retrieval/web/tavily.go`
- `open-webui/backend/open_webui/retrieval/web/utils.py` -> `backend/open_webui/retrieval/web/utils.go`
- `open-webui/backend/open_webui/retrieval/web/yacy.py` -> `backend/open_webui/retrieval/web/yacy.go`
- `open-webui/backend/open_webui/retrieval/web/yandex.py` -> `backend/open_webui/retrieval/web/yandex.go`
- `open-webui/backend/open_webui/retrieval/web/ydc.py` -> `backend/open_webui/retrieval/web/ydc.go`

## routers
- `open-webui/backend/open_webui/routers/analytics.py` -> `backend/open_webui/routers/analytics.go`
- `open-webui/backend/open_webui/routers/audio.py` -> `backend/open_webui/routers/audio.go`
- `open-webui/backend/open_webui/routers/auths.py` -> `backend/open_webui/routers/auths.go`
- `open-webui/backend/open_webui/routers/channels.py` -> `backend/open_webui/routers/channels.go`
- `open-webui/backend/open_webui/routers/chats.py` -> `backend/open_webui/routers/chats.go`
- `open-webui/backend/open_webui/routers/configs.py` -> `backend/open_webui/routers/configs.go`
- `open-webui/backend/open_webui/routers/evaluations.py` -> `backend/open_webui/routers/evaluations.go`
- `open-webui/backend/open_webui/routers/files.py` -> `backend/open_webui/routers/files.go`
- `open-webui/backend/open_webui/routers/folders.py` -> `backend/open_webui/routers/folders.go`
- `open-webui/backend/open_webui/routers/functions.py` -> `backend/open_webui/routers/functions.go`
- `open-webui/backend/open_webui/routers/groups.py` -> `backend/open_webui/routers/groups.go`
- `open-webui/backend/open_webui/routers/images.py` -> `backend/open_webui/routers/images.go`
- `open-webui/backend/open_webui/routers/knowledge.py` -> `backend/open_webui/routers/knowledge.go`
- `open-webui/backend/open_webui/routers/memories.py` -> `backend/open_webui/routers/memories.go`
- `open-webui/backend/open_webui/routers/models.py` -> `backend/open_webui/routers/models.go`
- `open-webui/backend/open_webui/routers/notes.py` -> `backend/open_webui/routers/notes.go`
- `open-webui/backend/open_webui/routers/ollama.py` -> `backend/open_webui/routers/ollama.go`
- `open-webui/backend/open_webui/routers/openai.py` -> `backend/open_webui/routers/openai.go`
- `open-webui/backend/open_webui/routers/pipelines.py` -> `backend/open_webui/routers/pipelines.go`
- `open-webui/backend/open_webui/routers/prompts.py` -> `backend/open_webui/routers/prompts.go`
- `open-webui/backend/open_webui/routers/retrieval.py` -> `backend/open_webui/routers/retrieval.go`
- `open-webui/backend/open_webui/routers/scim.py` -> `backend/open_webui/routers/scim.go`
- `open-webui/backend/open_webui/routers/skills.py` -> `backend/open_webui/routers/skills.go`
- `open-webui/backend/open_webui/routers/tasks.py` -> `backend/open_webui/routers/tasks.go`
- `open-webui/backend/open_webui/routers/terminals.py` -> `backend/open_webui/routers/terminals.go`
- `open-webui/backend/open_webui/routers/tools.py` -> `backend/open_webui/routers/tools.go`
- `open-webui/backend/open_webui/routers/users.py` -> `backend/open_webui/routers/users.go`
- `open-webui/backend/open_webui/routers/utils.py` -> `backend/open_webui/routers/utils.go`

## socket
- `open-webui/backend/open_webui/socket/main.py` -> `backend/open_webui/socket/main.go`
- `open-webui/backend/open_webui/socket/utils.py` -> `backend/open_webui/socket/utils.go`

## storage
- `open-webui/backend/open_webui/storage/provider.py` -> `backend/open_webui/storage/provider.go`

## test
- `open-webui/backend/open_webui/test/__init__.py` -> `backend/open_webui/test/init__.go`
- `open-webui/backend/open_webui/test/apps/webui/routers/test_auths.py` -> `backend/open_webui/test/apps/webui/routers/test_auths_test.go`
- `open-webui/backend/open_webui/test/apps/webui/routers/test_models.py` -> `backend/open_webui/test/apps/webui/routers/test_models_test.go`
- `open-webui/backend/open_webui/test/apps/webui/routers/test_users.py` -> `backend/open_webui/test/apps/webui/routers/test_users_test.go`
- `open-webui/backend/open_webui/test/apps/webui/storage/test_provider.py` -> `backend/open_webui/test/apps/webui/storage/test_provider_test.go`
- `open-webui/backend/open_webui/test/util/test_redis.py` -> `backend/open_webui/test/util/test_redis_test.go`

## tools
- `open-webui/backend/open_webui/tools/__init__.py` -> `backend/open_webui/tools/init__.go`
- `open-webui/backend/open_webui/tools/builtin.py` -> `backend/open_webui/tools/builtin.go`

## utils
- `open-webui/backend/open_webui/utils/access_control/__init__.py` -> `backend/open_webui/utils/access_control/init__.go`
- `open-webui/backend/open_webui/utils/access_control/files.py` -> `backend/open_webui/utils/access_control/files.go`
- `open-webui/backend/open_webui/utils/actions.py` -> `backend/open_webui/utils/actions.go`
- `open-webui/backend/open_webui/utils/anthropic.py` -> `backend/open_webui/utils/anthropic.go`
- `open-webui/backend/open_webui/utils/audit.py` -> `backend/open_webui/utils/audit.go`
- `open-webui/backend/open_webui/utils/auth.py` -> `backend/open_webui/utils/auth.go`
- `open-webui/backend/open_webui/utils/channels.py` -> `backend/open_webui/utils/channels.go`
- `open-webui/backend/open_webui/utils/chat.py` -> `backend/open_webui/utils/chat.go`
- `open-webui/backend/open_webui/utils/code_interpreter.py` -> `backend/open_webui/utils/code_interpreter.go`
- `open-webui/backend/open_webui/utils/embeddings.py` -> `backend/open_webui/utils/embeddings.go`
- `open-webui/backend/open_webui/utils/files.py` -> `backend/open_webui/utils/files.go`
- `open-webui/backend/open_webui/utils/filter.py` -> `backend/open_webui/utils/filter.go`
- `open-webui/backend/open_webui/utils/groups.py` -> `backend/open_webui/utils/groups.go`
- `open-webui/backend/open_webui/utils/headers.py` -> `backend/open_webui/utils/headers.go`
- `open-webui/backend/open_webui/utils/images/comfyui.py` -> `backend/open_webui/utils/images/comfyui.go`
- `open-webui/backend/open_webui/utils/logger.py` -> `backend/open_webui/utils/logger.go`
- `open-webui/backend/open_webui/utils/mcp/client.py` -> `backend/open_webui/utils/mcp/client.go`
- `open-webui/backend/open_webui/utils/middleware.py` -> `backend/open_webui/utils/middleware.go`
- `open-webui/backend/open_webui/utils/misc.py` -> `backend/open_webui/utils/misc.go`
- `open-webui/backend/open_webui/utils/models.py` -> `backend/open_webui/utils/models.go`
- `open-webui/backend/open_webui/utils/oauth.py` -> `backend/open_webui/utils/oauth.go`
- `open-webui/backend/open_webui/utils/payload.py` -> `backend/open_webui/utils/payload.go`
- `open-webui/backend/open_webui/utils/pdf_generator.py` -> `backend/open_webui/utils/pdf_generator.go`
- `open-webui/backend/open_webui/utils/plugin.py` -> `backend/open_webui/utils/plugin.go`
- `open-webui/backend/open_webui/utils/rate_limit.py` -> `backend/open_webui/utils/rate_limit.go`
- `open-webui/backend/open_webui/utils/redis.py` -> `backend/open_webui/utils/redis.go`
- `open-webui/backend/open_webui/utils/response.py` -> `backend/open_webui/utils/response.go`
- `open-webui/backend/open_webui/utils/sanitize.py` -> `backend/open_webui/utils/sanitize.go`
- `open-webui/backend/open_webui/utils/security_headers.py` -> `backend/open_webui/utils/security_headers.go`
- `open-webui/backend/open_webui/utils/task.py` -> `backend/open_webui/utils/task.go`
- `open-webui/backend/open_webui/utils/telemetry/__init__.py` -> `backend/open_webui/utils/telemetry/init__.go`
- `open-webui/backend/open_webui/utils/telemetry/constants.py` -> `backend/open_webui/utils/telemetry/constants.go`
- `open-webui/backend/open_webui/utils/telemetry/instrumentors.py` -> `backend/open_webui/utils/telemetry/instrumentors.go`
- `open-webui/backend/open_webui/utils/telemetry/logs.py` -> `backend/open_webui/utils/telemetry/logs.go`
- `open-webui/backend/open_webui/utils/telemetry/metrics.py` -> `backend/open_webui/utils/telemetry/metrics.go`
- `open-webui/backend/open_webui/utils/telemetry/setup.py` -> `backend/open_webui/utils/telemetry/setup.go`
- `open-webui/backend/open_webui/utils/tools.py` -> `backend/open_webui/utils/tools.go`
- `open-webui/backend/open_webui/utils/validate.py` -> `backend/open_webui/utils/validate.go`
- `open-webui/backend/open_webui/utils/webhook.py` -> `backend/open_webui/utils/webhook.go`

## 12. 执行清单

- [x] 12.1 在 `REFACTOR_PLAN.md` 末尾追加可执行 checklist
- [x] 12.2 实现 `backend/scripts/sync_inventory.sh`，生成 `SYNC_MAP` 与 Python 基线清单
- [x] 12.3 基于清单批量生成 `backend/open_webui` 镜像目录与 `.go` stub
- [x] 12.4 实现 `backend/cmd/openwebui/main.go`
- [x] 12.5 实现 `backend/open_webui/env.go`
- [x] 12.6 实现 `backend/open_webui/main.go`
- [x] 12.7 实现 `backend/internal/platform/proxy/proxy.go`
- [x] 12.8 运行清单生成与 stub 生成
- [x] 12.9 运行 `go test ./...`
- [x] 12.10 回写已完成勾选状态
- [x] 12.11 扩展 `backend/open_webui/env.go`，补齐 DB 相关运行配置
- [x] 12.12 实现 `backend/open_webui/internal/db.go` 的最小数据库连接层
- [x] 12.13 实现 `backend/open_webui/config.go` 的最小配置存取层
- [x] 12.14 为 `config.go` 与 `internal/db.go` 补最小测试
- [x] 12.15 运行 `go mod tidy`
- [x] 12.16 再次运行 `go test ./...`
- [x] 12.17 实现 `backend/open_webui/migrations/env.go` 的最小 migration runner
- [x] 12.18 实现 `backend/open_webui/migrations/util.go` 的表枚举能力
- [x] 12.19 实现 `backend/open_webui/migrations/versions/ca81bd47c050_add_config_table.go`
- [x] 12.20 将 migration 接入 `backend/open_webui/config.go`
- [x] 12.21 为 migration 增加最小测试
- [x] 12.22 再次运行 `go test ./...`
- [x] 12.23 实现 `backend/open_webui/migrations/versions/7e5b5dc7342b_init.go`
- [x] 12.24 扩展 migration 测试，验证基础表与 config 表创建
- [x] 12.25 再次运行 `go test ./...`
- [x] 12.26 实现 `backend/open_webui/internal/wrappers.go`
- [x] 12.27 为 `internal/wrappers.go` 增加最小测试
- [x] 12.28 再次运行 `go test ./...`
- [x] 12.29 实现 `backend/open_webui/models/users.go` 的最小仓储层
- [x] 12.30 实现 `backend/open_webui/models/auths.go` 的最小仓储层
- [x] 12.31 为 `models/users.go` 与 `models/auths.go` 增加最小测试
- [x] 12.32 更新 `backend/SYNC_MAP.yaml` 中 `models/users.py` 与 `models/auths.py` 状态
- [x] 12.33 再次运行 `go test ./...`
- [x] 12.34 扩展 `backend/open_webui/models/users.go`，补用户计数能力
- [x] 12.35 实现 `backend/open_webui/utils/auth.go` 的最小密码与 JWT 能力
- [x] 12.36 实现 `backend/open_webui/routers/auths.go` 的最小 `signup/signin` 路由
- [x] 12.37 将 `signup/signin` 接入 `backend/open_webui/main.go`
- [x] 12.38 为 auth util 与 auth router 增加最小测试
- [x] 12.39 更新 `backend/SYNC_MAP.yaml` 中 `utils/auth.py` 与 `routers/auths.py` 状态
- [x] 12.40 运行 `go mod tidy`
- [x] 12.41 再次运行 `go test ./...`
- [x] 12.42 扩展 `backend/open_webui/utils/auth.go`，补 token 提取能力
- [x] 12.43 扩展 `backend/open_webui/routers/auths.go`，补 session user 与 signout
- [x] 12.44 为新增 auth 路由补测试
- [x] 12.45 再次运行 `go test ./...`
- [x] 12.46 实现 `backend/open_webui/utils/validate.go`
- [x] 12.47 实现 `backend/open_webui/routers/users.go` 的最小 `info/profile image` 路由
- [x] 12.48 将 users 路由接入 `backend/open_webui/main.go`
- [x] 12.49 为 users 路由补最小测试
- [x] 12.50 更新 `backend/SYNC_MAP.yaml` 中 `utils/validate.py` 与 `routers/users.py` 状态
- [x] 12.51 再次运行 `go test ./...`
- [x] 12.52 扩展 `backend/open_webui/models/users.go`，补 settings/info 更新能力
- [x] 12.53 扩展 `backend/open_webui/routers/users.go`，补 session user settings/info 读写
- [x] 12.54 为新增 users 路由补测试
- [x] 12.55 再次运行 `go test ./...`
- [x] 12.56 实现 `backend/open_webui/migrations/versions/3af16a1c9fb6_update_user_table.go`
- [x] 12.57 实现 `backend/open_webui/migrations/versions/b10670c03dd5_update_user_table.go`
- [x] 12.58 实现 `backend/open_webui/migrations/versions/b2c3d4e5f6a7_add_scim_column_to_user_table.go`
- [x] 12.59 将新增用户迁移接入 `backend/open_webui/migrations/env.go`
- [x] 12.60 扩展 `backend/open_webui/models/users.go` 以读取新增用户字段
- [x] 12.61 扩展 users/auths 相关测试覆盖新 schema
- [x] 12.62 更新 `backend/SYNC_MAP.yaml` 中三个用户迁移状态
- [x] 12.63 再次运行 `go test ./...`
- [x] 12.64 扩展 `backend/open_webui/models/users.go`，补 `GetFirstUser`
- [x] 12.65 扩展 `backend/open_webui/routers/auths.go`，补 profile/timezone/password 更新
- [x] 12.66 扩展 `backend/open_webui/routers/users.go`，补管理员用户更新
- [x] 12.67 为新增更新接口补测试
- [x] 12.68 再次运行 `go test ./...`
- [x] 12.69 扩展 `backend/open_webui/routers/users.go`，补管理员 get/delete user
- [x] 12.70 统一 users 路由的用户响应结构
- [x] 12.71 为 get/delete user 补测试
- [x] 12.72 再次运行 `go test ./...`
- [x] 12.73 扩展 `backend/open_webui/routers/users.go`，补 user status/active 路由
- [x] 12.74 为 user status/active 补测试
- [x] 12.75 再次运行 `go test ./...`
- [x] 12.76 扩展 `backend/open_webui/models/users.go`，补 active 判定能力
- [x] 12.77 细化 `backend/open_webui/routers/users.go` 的 info/active/profile image 返回
- [x] 12.78 为细化后的 users 路由补测试
- [x] 12.79 再次运行 `go test ./...`
- [x] 12.80 扩展 `backend/open_webui/models/users.go`，补 api_key 增删改查
- [x] 12.81 扩展 `backend/open_webui/utils/auth.go`，补 api key 生成能力
- [x] 12.82 扩展 `backend/open_webui/routers/auths.go`，补 `GET/POST/DELETE /api_key`
- [x] 12.83 为 api_key 接口补测试
- [x] 12.84 再次运行 `go test ./...`
- [x] 12.85 将 api key 接入当前用户解析
- [x] 12.86 扩展 auths/users 路由测试，验证 api key 可用于鉴权
- [x] 12.87 再次运行 `go test ./...`
- [x] 12.88 扩展 `backend/open_webui/routers/auths.go`，补管理员 add user
- [x] 12.89 扩展 `backend/open_webui/routers/auths.go`，补 `admin/details`
- [x] 12.90 扩展运行配置，补 `SHOW_ADMIN_DETAILS` 与 `ADMIN_EMAIL`
- [x] 12.91 为新增管理员接口补测试
- [x] 12.92 再次运行 `go test ./...`
- [x] 12.93 实现 `backend/open_webui/migrations/versions/922e7a387820_add_group_table.go`
- [x] 12.94 实现 `backend/open_webui/migrations/versions/37f288994c47_add_group_member_table.go`
- [x] 12.95 将 group 相关迁移接入 `backend/open_webui/migrations/env.go`
- [x] 12.96 实现 `backend/open_webui/models/groups.go` 的最小仓储层
- [x] 12.97 扩展 `backend/open_webui/models/users.go`，补 `get_valid_user_ids`
- [x] 12.98 实现 `backend/open_webui/routers/groups.go` 的最小 CRUD 与成员管理
- [x] 12.99 将 `groups` 路由接入 `backend/open_webui/main.go`
- [x] 12.100 为 groups 增加最小测试
- [x] 12.101 更新 `backend/SYNC_MAP.yaml` 中 group 相关文件状态
- [x] 12.102 再次运行 `go test ./...`
- [x] 12.103 扩展 `backend/open_webui/models/users.go`，补按 group/user 查询成员
- [x] 12.104 扩展 `backend/open_webui/routers/users.go`，补 `/{user_id}/groups`
- [x] 12.105 扩展 `backend/open_webui/routers/groups.go`，补 `/id/{id}/users`
- [x] 12.106 将 users info/get 响应接入真实 groups 数据
- [x] 12.107 为成员查询接口补测试
- [x] 12.108 再次运行 `go test ./...`
- [x] 12.109 实现 `backend/open_webui/migrations/versions/38d63c18f30f_add_oauth_session_table.go`
- [x] 12.110 将 oauth session 迁移接入 `backend/open_webui/migrations/env.go`
- [x] 12.111 实现 `backend/open_webui/models/oauth_sessions.go` 的最小仓储层
- [x] 12.112 扩展 `backend/open_webui/routers/users.go`，补 `/{user_id}/oauth/sessions`
- [x] 12.113 在 `main.go` 接入 oauth session 依赖
- [x] 12.114 为 oauth session 路由补测试
- [x] 12.115 更新 `backend/SYNC_MAP.yaml` 中 oauth session 相关文件状态
- [x] 12.116 再次运行 `go test ./...`
- [x] 12.117 扩展 `backend/open_webui/models/users.go`，补用户列表/搜索能力
- [x] 12.118 扩展 `backend/open_webui/routers/users.go`，补 `/` `/all` `/search` `/groups`
- [x] 12.119 为 users 列表接口补测试
- [x] 12.120 再次运行 `go test ./...`
- [x] 12.121 扩展 `backend/open_webui/routers/groups.go`，补 `/id/{id}/export`
- [x] 12.122 为 groups export 补测试
- [x] 12.123 再次运行 `go test ./...`
- [x] 12.124 扩展运行配置，补 `ENABLE_SIGNUP` 与 `DEFAULT_USER_ROLE`
- [x] 12.125 扩展 `backend/open_webui/routers/auths.go`，补 `GET /admin/config`
- [x] 12.126 为 `admin/config` 补测试
- [x] 12.127 再次运行 `go test ./...`
- [x] 12.128 实现 `backend/open_webui/migrations/versions/9f0c9cd09105_add_note_table.go`
- [x] 12.129 将 note 迁移接入 `backend/open_webui/migrations/env.go`
- [x] 12.130 实现 `backend/open_webui/models/notes.go` 的最小仓储层
- [x] 12.131 实现 `backend/open_webui/routers/notes.go` 的最小 CRUD
- [x] 12.132 在 `main.go` 接入 notes 路由
- [x] 12.133 为 notes 增加最小测试
- [x] 12.134 更新 `backend/SYNC_MAP.yaml` 中 note 相关文件状态
- [x] 12.135 再次运行 `go test ./...`
- [x] 12.136 实现 `backend/open_webui/models/memories.go` 的最小仓储层
- [x] 12.137 实现 `backend/open_webui/routers/memories.go` 的最小 CRUD
- [x] 12.138 在 `main.go` 接入 memories 路由
- [x] 12.139 为 memories 增加最小测试
- [x] 12.140 更新 `backend/SYNC_MAP.yaml` 中 memory 相关文件状态
- [x] 12.141 再次运行 `go test ./...`
- [x] 12.142 实现 folder 相关迁移文件
- [x] 12.143 实现 `backend/open_webui/models/folders.go` 的最小仓储层
- [x] 12.144 实现 `backend/open_webui/routers/folders.go` 的最小 CRUD
- [x] 12.145 在 `main.go` 接入 folders 路由
- [x] 12.146 为 folders 增加最小测试
- [x] 12.147 更新 `backend/SYNC_MAP.yaml` 中 folder 相关文件状态
- [x] 12.148 再次运行 `go test ./...`
