---
name: guard-go-ddd-layers
description: 守护 Go 项目的 DDD 分层和业务不变量归属。用于新增、修改或审查 domain、application、repository、infrastructure、HTTP handler、事务用例和带状态 CRUD，尤其适用于避免贫血领域模型、透传 application 和承载业务规则的胖 repository。
---

# Go DDD 分层守卫

在编码前确定业务规则属于哪一层，编码后检查规则没有下沉到基础设施层。

## 目录与包组织

优先按限界上下文纵向组织代码，再在上下文内部按职责分层；不要在 `internal` 下建立覆盖全项目的
`domain`、`application`、`infrastructure`、`transport` 四棵横向大目录。

```text
internal/<context>/
├── domain/              # 实体、值对象、领域服务、领域端口和领域错误
├── application/         # 用例、事务编排及由用例消费的端口
├── infrastructure/      # 数据库、消息、缓存和外部服务适配器
└── transport/http/      # HTTP 请求响应模型、参数校验和错误映射
```

- `platform` 只承载数据库连接、配置、日志、指标、ID 等无业务语义的通用技术能力。
- `shared` 只放真正跨上下文且稳定的最小共享内核，不能成为业务代码的兜底目录。
- Go 的包边界由目录决定；同一个包内用多个文件表达模型、策略、用例或适配器，不为每个类型、聚合或用例创建子目录。
- 文件按职责内聚拆分，而不是机械地“一类型一文件”。只有文件已经承载多个独立概念、明显难以导航时才拆分；强相关且规模较小的模型与行为可以放在同一文件。
- 只有出现独立依赖方向、可替换适配器或需要单独封装的实现时，才在 `infrastructure` 下继续建立 `mysql`、`security`、`query` 等子包。
- 同一上下文可以同时包含多个用例，例如学生认证和管理员认证仍属于 `identity`；不要仅因调用方不同就复制一套分层目录。
- 包名表达业务概念，可与职责目录名不同，例如 `internal/identity/domain` 使用 `package auth`；调用方通过清晰的 import alias 表达其角色。

## 编码前

先列出本次需求的业务不变量，并为每条规则标注唯一的主要归属：

- Transport：JSON、路径参数、数据类型、必填和格式校验。
- Domain：业务合法性、状态迁移、实体行为、跨实体业务策略。
- Application：加载聚合、事务边界、权限检查、调用领域行为、协调端口。
- Infrastructure：查询、持久化、外部系统适配、锁和条件写入。

给出简短调用链后再改代码：

```text
Handler → Application command → Repository load → Domain behavior → Repository save
```

## 硬性规则

- 禁止让 Application 只是 Repository 的同名方法转发器。
- 禁止把 `state == planned`、能否删除、能否绑定等业务判断只写在 Repository。
- 禁止由 Application 或 Infrastructure 任意修改领域状态；通过实体行为或领域服务完成。
- 将 Repository 接口定义在消费它的 Domain 或 Application 层，不让上层依赖具体数据库实现。
- 同一消费包的 Repository 接口集中放在独立的 `repository.go`；Application 消费的端口留在 Application，不为统一目录而下沉到 Domain。
- 将事务编排放在 Application；让 Domain 保持与数据库框架无关。
- 允许 Infrastructure 使用行锁、版本号和条件更新防止并发漂移，但它们只是领域规则的并发兜底。
- 条件写入影响零行时返回并发冲突；不要在 Infrastructure 中重新发明业务含义。
- 删除依赖是否存在是 Repository 提供的事实；根据事实决定能否删除是 Domain 策略。
- 只有完全没有状态和业务规则的参考数据才允许使用简单 CRUD transaction script。

## 领域错误维护

- 每个 Domain 包在独立的 `error.go` 中集中维护领域错误，不在模型或服务文件中散落 `errors.New`。
- 面向用户和日志的错误信息统一使用中文；稳定的机器错误码可以保留英文标识。
- 仅跨包判断的错误才导出，并使用哨兵错误和 `%w` 保留 `errors.Is` 错误链。
- Domain 错误不包含 HTTP 状态码、响应结构等 Transport 语义。

## Go 可见性

- 函数、方法和类型默认不导出；只有真实的跨包调用、接口实现或框架约定需要时才使用大写名称。
- 审计导出符号时检查整个目标包及全仓引用，不能只检查当前文件；测试代码不能成为扩大生产 API 的唯一理由。
- `TableName` 等反射约定属于例外，需在交付说明中明确保留原因。

## 推荐形态

```go
// Domain：规则的唯一业务表达。
func (c *TeachingClass) ChangePlan(cmd ChangePlan) error {
	if c.State != StatePlanned {
		return ErrTeachingClassNotEditable
	}
	// 校验容量、年级和时间安排后修改实体。
	return nil
}

// Application：事务和用例编排。
func (h *ChangeTeachingClassHandler) Handle(ctx context.Context, cmd Command) error {
	return h.tx.Within(ctx, func(ctx context.Context) error {
		class, err := h.classes.GetForUpdate(ctx, cmd.ID)
		if err != nil {
			return err
		}
		if err := class.ChangePlan(cmd.Change); err != nil {
			return err
		}
		return h.classes.Save(ctx, class)
	})
}
```

Repository 仍可执行 `UPDATE ... WHERE state = 'planned'`，但 Domain 必须已经表达“为什么只有 planned 可以修改”。

## 编码后审计

逐项检查并在交付说明中简要报告：

1. 新代码是否落在正确的限界上下文，且没有形成全局横向分层或无意义的细粒度子包。
2. Domain 是否包含本次新增的业务行为和错误语义。
3. Application 是否形成完整用例，而非透传 CRUD。
4. Infrastructure 是否只保留持久化与并发实现细节。
5. 同一业务规则是否有明确的唯一来源。
6. 是否测试领域规则、Application 编排和 Repository 并发冲突。

发现贫血 Domain、透传 Application 或胖 Repository 时，先重构分层再宣告完成。
