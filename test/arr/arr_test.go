package arr_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/alert666/api-server/model"
)

func TestArr(t *testing.T) {
	var a []int
	b := []int{}

	fmt.Println(a == nil)
	fmt.Println(b == nil)

	fmt.Println(len(a))
	fmt.Println(len(b))
}

func TestSplit(t *testing.T) {
	a := "/api/v1/user/login"
	a = strings.TrimPrefix(a, "/")
	ty := strings.Split(a, "/")[2]
	fmt.Println(ty)
}

// 测试稳定性的单元测试
func TestSortStableFuncStability(t *testing.T) {
	// 创建具有相同创建时间但不同 Node 的对象
	baseTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	obj := []*model.IDCHeartbeat{
		{CreatedAt: baseTime, Node: "node1", IP: "192.168.1.1"},
		{CreatedAt: baseTime.Add(time.Hour * 20), Node: "node1", IP: "192.168.1.1"},
		{CreatedAt: baseTime.Add(-time.Hour), Node: "node2", IP: "192.168.1.2"},
		{CreatedAt: baseTime, Node: "node3", IP: "192.168.1.3"},
		{CreatedAt: baseTime.Add(-2 * time.Hour), Node: "node4", IP: "192.168.1.4"},
	}

	slices.SortStableFunc(obj, func(a, b *model.IDCHeartbeat) int {
		return b.CreatedAt.Compare(a.CreatedAt)
	})

	for _, v := range obj {
		fmt.Printf("v.CreatedAt: %v\n", v.CreatedAt)
	}
}
