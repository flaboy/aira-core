# Aira Core - Quick Start

快速开始指南

## 简介

Aira Core 是一个强大的 Go 框架核心包。

## 安装

```bash
go get github.com/yourorg/aira-core
```

## 基本用法

```go
package main

import (
    "github.com/yourorg/aira-core"
)

func main() {
    core := aira.New()
    core.Start()
}
```
