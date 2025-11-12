# Bootstrap Module Initialization Framework

## Overview

Bootstrap is a modular initialization framework that lets modules register hooks to automatically finish their own setup. It supports priority control, conditional initialization, flexible error handling strategies, detailed logging, thread safety, and test-friendly cleanup.

## Key Features

- ✅ **Priority ordering** – ensures modules initialize in the correct sequence
- ✅ **Named hooks** – clear names make logs easy to trace and debug
- ✅ **Context sharing** – modules can exchange data such as database handles
- ✅ **Conditional initialization** – toggle module startup based on configuration
- ✅ **Flexible error handling** – choose fail-fast, continue, or warn behaviors
- ✅ **Verbose logging** – shows progress and timing for each hook
- ✅ **Thread-safe** – mutex-protected global state
- ✅ **Test friendly** – `Clear()` removes all registered hooks

## Quick Start

### 1. Register an initialization hook inside your module

Create an `init.go` file in your module package:

```go
// proxy/init.go
package proxy

import (
	"nofx/bootstrap"
	"nofx/config"
)

func init() {
	bootstrap.Register("Proxy Module", bootstrap.PriorityCore, initProxyModule)
}

func initProxyModule(ctx *bootstrap.Context) error {
	proxyConfig := ctx.Config.Proxy

	if err := InitGlobalProxyManager(proxyConfig); err != nil {
		return err
	}

	ctx.Set("proxy_manager", GetGlobalProxyManager())

	return nil
}
```

### 2. Run the initialization in `main.go`

```go
package main

import (
	"log"
	"nofx/bootstrap"
	"nofx/config"

	_ "nofx/proxy"
	_ "nofx/market"
	_ "nofx/trader"
)

func main() {
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx := bootstrap.NewContext(cfg)

	if err := bootstrap.Run(ctx); err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}

	// Start your business logic...
}
```

### 3. Expected output

```
🔄 Starting initialization of 3 modules...
  [1/3] Initializing: Database Module (priority: 20)
  ✓ Done: Database Module (elapsed: 120ms)
  [2/3] Initializing: Proxy Module (priority: 50)
    ↳ Proxy auto-refresh started (interval: 30m0s)
    ↳ Proxy pool status: total=5, blacklist=0, available=5
  ✓ Done: Proxy Module (elapsed: 35ms)
  [3/3] Initializing: Market Module (priority: 100)
  ✓ Done: Market Module (elapsed: 200ms)
✅ All modules initialized (total: 355ms)
📊 Stats: success=3, skipped=0
```

## Priority Constants

The system defines the following priority constants (smaller numbers execute earlier):

| Constant | Value | Purpose | Example |
|----------|-------|---------|---------|
| `PriorityInfrastructure` | 10 | Infrastructure | Logging, config loading |
| `PriorityDatabase` | 20 | Database connections | SQLite, Redis |
| `PriorityCore` | 50 | Core modules | Proxy, Market Monitor |
| `PriorityBusiness` | 100 | Business modules | Trader, API Server |
| `PriorityBackground` | 200 | Background tasks | Schedulers, monitoring |

### Usage example

```go
// Database module (initializes first)
bootstrap.Register("Database", bootstrap.PriorityDatabase, initDatabase)

// Proxy module (core module)
bootstrap.Register("Proxy", bootstrap.PriorityCore, initProxy)

// Trader module (depends on database and proxy)
bootstrap.Register("Trader", bootstrap.PriorityBusiness, initTrader)
```

## Advanced Features

### 1. Conditional initialization

Some modules only need to start when certain conditions are met:

```go
bootstrap.Register("Proxy Module", bootstrap.PriorityCore, initProxy).
	EnabledIf(func(ctx *bootstrap.Context) bool {
		return ctx.Config.Proxy != nil && ctx.Config.Proxy.Enabled
	})
```

**Output**:
```
  [2/5] Skipped: Proxy Module (condition not met)
```

### 2. Error handling policies

Three strategies are available:

#### FailFast (default) – stop immediately on error

```go
bootstrap.Register("Database", bootstrap.PriorityDatabase, initDatabase)
// FailFast is the default; no extra configuration needed
```

**Effect**: if Database initialization fails, system startup stops.

#### ContinueOnError – keep going, aggregate all errors

```go
bootstrap.Register("Proxy", bootstrap.PriorityCore, initProxy).
	OnError(bootstrap.ContinueOnError)
```

**Effect**: Proxy failure doesn’t block other modules; all errors are reported at the end.

#### WarnOnError – continue, log a warning

```go
bootstrap.Register("Proxy", bootstrap.PriorityCore, initProxy).
	OnError(bootstrap.WarnOnError)
```

**Effect**: Proxy failure emits a warning and the system keeps running.

**Output**:
```
  [2/5] Initializing: Proxy Module (priority: 50)
  ⚠️  Warning: Proxy Module (elapsed: 15ms) – failed to connect to proxy server
```

### 3. Context data sharing

Modules can share data through the initialization context:

```go
// database/init.go – store the database instance
func initDatabase(ctx *bootstrap.Context) error {
	db, err := sql.Open("sqlite", "config.db")
	if err != nil {
		return err
	}

	ctx.Set("database", db)
	return nil
}

// trader/init.go – retrieve the database instance
func initTrader(ctx *bootstrap.Context) error {
	db, ok := ctx.Get("database")
	if !ok {
		return fmt.Errorf("database not initialized")
	}

	database := db.(*sql.DB)
	// Use database to initialize trader...
	return nil
}
```

**Safe access**:
```go
// Use MustGet for required dependencies; it panics if missing
db := ctx.MustGet("database").(*sql.DB)
```

### 4. Fluent chaining

Chain configuration methods for better readability:

```go
bootstrap.Register("Proxy", bootstrap.PriorityCore, initProxy).
	EnabledIf(func(ctx *bootstrap.Context) bool {
		return ctx.Config.Proxy != nil && ctx.Config.Proxy.Enabled
	}).
	OnError(bootstrap.WarnOnError)
```

### 5. Custom default error policy

Specify a default policy when running all hooks:

```go
// All hooks default to ContinueOnError unless overridden
err := bootstrap.RunWithPolicy(ctx, bootstrap.ContinueOnError)
```

## Complete Examples

### Example 1: Database module

```go
// database/init.go
package database

import (
	"database/sql"
	"nofx/bootstrap"
)

func init() {
	bootstrap.Register("Database", bootstrap.PriorityDatabase, initDatabase)
}

func initDatabase(ctx *bootstrap.Context) error {
	db, err := sql.Open("sqlite", "config.db")
	if err != nil {
		return err
	}

	if err := db.Ping(); err != nil {
		return err
	}

	ctx.Set("database", db)
	return nil
}
```

### Example 2: Proxy module (conditional + warning policy)

```go
// proxy/init.go
package proxy

import (
	"nofx/bootstrap"
	"nofx/config"
)

func init() {
	bootstrap.Register("Proxy", bootstrap.PriorityCore, initProxy).
		EnabledIf(func(ctx *bootstrap.Context) bool {
			return ctx.Config.Proxy != nil && ctx.Config.Proxy.Enabled
		}).
		OnError(bootstrap.WarnOnError) // Proxy failure does not stop the system
}

func initProxy(ctx *bootstrap.Context) error {
	proxyConfig := convertConfig(ctx.Config.Proxy)

	if err := InitGlobalProxyManager(proxyConfig); err != nil {
		return err
	}

	ctx.Set("proxy_manager", GetGlobalProxyManager())
	return nil
}
```

### Example 3: Trader module (depends on other modules)

```go
// trader/init.go
package trader

import (
	"nofx/bootstrap"
)

func init() {
	bootstrap.Register("Trader", bootstrap.PriorityBusiness, initTrader)
}

func initTrader(ctx *bootstrap.Context) error {
	db := ctx.MustGet("database").(*sql.DB)

	var proxyMgr *proxy.ProxyManager
	if pm, ok := ctx.Get("proxy_manager"); ok {
		proxyMgr = pm.(*proxy.ProxyManager)
	}

	// Initialize trader with dependencies...
	return nil
}
```

## Debugging and Testing

### Inspect registered hooks

```go
hooks := bootstrap.GetRegistered()
for _, hook := range hooks {
	fmt.Printf("Hook: %s, Priority: %d\n", hook.Name, hook.Priority)
}
```

### Clear hooks (for tests)

```go
func TestMyModule(t *testing.T) {
	bootstrap.Clear()

	bootstrap.Register("Test", 10, func(ctx *bootstrap.Context) error {
		return nil
	})

	// Run tests...
}
```

### Count hooks

```go
count := bootstrap.Count()
fmt.Printf("%d initialization hooks registered\n", count)
```

## Error Handling Best Practices

### 1. Use FailFast for critical modules

```go
// Database is critical; stop if it fails
bootstrap.Register("Database", bootstrap.PriorityDatabase, initDatabase)
// FailFast is implied
```

### 2. Use WarnOnError for optional modules

```go
// Proxy is optional; a warning is enough
bootstrap.Register("Proxy", bootstrap.PriorityCore, initProxy).
	OnError(bootstrap.WarnOnError)
```

### 3. Use ContinueOnError for batched initialization

```go
// Load a batch of plugins and review all failures together
for _, plugin := range plugins {
	bootstrap.Register(plugin.Name, 150, plugin.Init).
		OnError(bootstrap.ContinueOnError)
}
```

## FAQ

### Q1: How do I guarantee Module A runs before Module B?

Control the priority values:
```go
bootstrap.Register("ModuleA", 50, initA)  // runs earlier
bootstrap.Register("ModuleB", 100, initB) // runs later
```

### Q2: How can I get detailed information when initialization fails?

Hook names are included in error messages:
```
Error: [Proxy Module] initialization failed: proxy server connection timed out
```

### Q3: Can I register hooks dynamically?

Yes, but registering inside `init()` is recommended:
```go
// Recommended: register in init() (runs at package load time)
func init() {
	bootstrap.Register("MyModule", 100, initModule)
}

// Not recommended: registering at runtime may cause ordering issues
func main() {
	bootstrap.Register("MyModule", 100, initModule)
}
```

### Q4: How do I access command-line arguments inside a hook?

Pass them through the context:
```go
// main.go
ctx := bootstrap.NewContext(cfg)
ctx.Set("args", os.Args)

// module/init.go
func initModule(ctx *bootstrap.Context) error {
	args := ctx.MustGet("args").([]string)
	// Use args...
}
```

## Performance Considerations

- Hook registration is thread-safe but incurs slight locking overhead
- Register hooks in `init()` to avoid runtime registration costs
- Hooks execute sequentially, not concurrently
- Each hook’s duration is logged

## License

This module is part of the internal NOFX project and follows the project’s overall license.
