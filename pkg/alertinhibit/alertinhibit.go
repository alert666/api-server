package alertinhibit

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

type InhibitMatcher struct {
	SourceMatchers []*Matcher
	TargetMatchers []*Matcher
	Equal          []string
}

type Matcher struct {
	Name  string `json:"name"`  // 标签名
	Value string `json:"value"` // 标签值
	Type  string `json:"type"`  // 操作符: =, !=, =~, !~
	Equal []string
}

func NewMatchers() ([]*InhibitMatcher, error) {
	inhibitRules, err := NewInhibitRules()
	if err != nil {
		return nil, err
	}

	inhibitMatchers := make([]*InhibitMatcher, 0, len(inhibitRules))

	for _, v := range inhibitRules {
		_inhibitMatcher := &InhibitMatcher{
			SourceMatchers: []*Matcher{},
			TargetMatchers: []*Matcher{},
			Equal:          []string{},
		}
		if sMatchers, err := TransformStringsToMatchers(v.SourceMatchers, v.Equal); err != nil {
			return nil, err
		} else {
			_inhibitMatcher.SourceMatchers = sMatchers

		}
		if tMatchers, err := TransformStringsToMatchers(v.TargetMatchers, v.Equal); err != nil {
			return nil, err
		} else {
			_inhibitMatcher.TargetMatchers = tMatchers
		}
		_inhibitMatcher.Equal = v.Equal
		inhibitMatchers = append(inhibitMatchers, _inhibitMatcher)
	}

	return inhibitMatchers, nil
}

// GetCondition 返回 GORM 的 Where 条件片段和参数
// 返回示例: ("alertname = ?", "节点磁盘空间不足")
// 或者针对 JSON: ("labels->>? = ?", "$.\"device\"", "eth0")
func (m *Matcher) GetCondition() (string, interface{}, error) {
	var columnExpr string

	// 1. 判断是物理列还是 JSON 标签
	// 这里的物理列要匹配你数据库实体的 gorm:"column:xxx" 定义
	switch m.Name {
	case "alertname", "severity", "instance", "cluster":
		columnExpr = m.Name
	default:
		// 如果是非物理列，使用 MySQL 的 JSON 提取操作符 ->>
		// 这里的 labels 对应你数据库的列名
		columnExpr = fmt.Sprintf("labels->>'$.\"%s\"'", m.Name)
	}

	// 2. 根据操作符映射 SQL 关键字
	switch m.Type {
	case "=":
		return fmt.Sprintf("%s = ?", columnExpr), m.Value, nil
	case "!=":
		return fmt.Sprintf("%s != ?", columnExpr), m.Value, nil
	case "=~":
		// MySQL 的正则匹配
		return fmt.Sprintf("%s REGEXP ?", columnExpr), m.Value, nil
	case "!~":
		// MySQL 的正则不匹配
		return fmt.Sprintf("%s NOT REGEXP ?", columnExpr), m.Value, nil
	default:
		return "", nil, fmt.Errorf("不支持的 matcher 类型: %s", m.Type)
	}
}

// InhibitRule 抑制规则结构体
type InhibitRule struct {
	SourceMatchers []string `mapstructure:"source_matchers" json:"source_matchers"`
	TargetMatchers []string `mapstructure:"target_matchers" json:"target_matchers"`
	Equal          []string `mapstructure:"equal" json:"equal"`
}

func NewInhibitRules() ([]*InhibitRule, error) {
	var rules []*InhibitRule
	// UnmarshalKey 会自动解析 alert.inhibit_rules 下的列表并填充到 slice 中
	err := viper.UnmarshalKey("alert.inhibit_rules", &rules)
	if err != nil {
		return nil, fmt.Errorf("解析抑制规则失败, %w", err)
	}
	return rules, nil
}

// 定义匹配正则，用于解析 label="value" 或 label=~"value" 等格式
// 分组1: 标签名, 分组2: 操作符, 分组3: 标签值(带或不带引号)
var matcherRE = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)\s*(=~|!~|!=|=)\s*["']?(.*?)["']?$`)

// ParseMatcher 将字符串解析为 Matcher 结构体
func ParseMatcher(input string) (*Matcher, error) {
	input = strings.TrimSpace(input)
	caps := matcherRE.FindStringSubmatch(input)
	if len(caps) != 4 {
		return nil, fmt.Errorf("无效的匹配器格式: %s", input)
	}

	return &Matcher{
		Name:  caps[1],
		Type:  caps[2],
		Value: caps[3],
	}, nil
}

// TransformStringsToMatchers 批量转换字符串数组
func TransformStringsToMatchers(inputs []string, equal []string) ([]*Matcher, error) {
	matchers := make([]*Matcher, 0, len(inputs))
	for i, s := range inputs {
		m, err := ParseMatcher(s)
		if err != nil {
			return nil, err
		}
		if i == 0 {
			m.Equal = equal
		}
		matchers = append(matchers, m)
	}
	return matchers, nil
}

type InhibitWhere struct {
	SourcesWhere []*Where
	TargetsWhere []*Where
	Equal        []string
}

type Where struct {
	ColumnExpr string
	Value      any
}

func (m *InhibitMatcher) Match() (*InhibitWhere, error) {
	w := &InhibitWhere{
		SourcesWhere: make([]*Where, 0),
		TargetsWhere: make([]*Where, 0),
		Equal:        []string{},
	}
	for i, m := range m.SourceMatchers {
		columnExpr, value, err := m.GetCondition()
		if err != nil {
			return nil, err
		}
		w.SourcesWhere = append(w.SourcesWhere, &Where{
			ColumnExpr: columnExpr,
			Value:      value,
		})

		if i == 0 {
			w.Equal = m.Equal
		}
	}

	for i, m := range m.TargetMatchers {
		columnExpr, value, err := m.GetCondition()
		if err != nil {
			return nil, err
		}
		w.TargetsWhere = append(w.TargetsWhere, &Where{
			ColumnExpr: columnExpr,
			Value:      value,
		})

		if i == 0 {
			w.Equal = m.Equal
		}
	}
	return w, nil
}
