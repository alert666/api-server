# prompt

```go
package v1

import (
	"context"

	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
)

type AlertTemplateServicer interface {
	CreateApi(ctx context.Context, req *types.ApiCreateRequest) error
	UpdateApi(ctx context.Context, req *types.ApiUpdateRequest) error
	DeleteApi(ctx context.Context, req *types.IDRequest) error
	QueryApi(ctx context.Context, req *types.IDRequest) (*model.Api, error)
	ListApi(ctx context.Context, pagination *types.ApiListRequest) (*types.ApiListResponse, error)
}

type AlertTemplateService struct{}

func AlertTemplateServicer() AlertTemplateServicer {
	return &AlertTemplateService{}
}
```

vscode 如何快速实现这些方法

在 VS Code 中，如果你安装了官方的 Go 扩展 (Go Team at Google)，有几种非常快的方法可以自动生成接口实现。

使用 Go: Generate Interface Stubs 命令（最推荐）, 这是最标准的方法，可以一次性生成所有方法的空壳。

按下键盘快捷键：Ctrl + Shift + P (Mac 为 Cmd + Shift + P)。输入并选择：Go: Generate Interface Stubs。在弹出的输入框中按照以下格式输入：

```go
recevicer *alertHistoryController AlertHistoryController
```

按下回车，所有方法就会自动插入到文件中
