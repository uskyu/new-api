package model

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/common"
)

// ValidateEmailNumericOnlyRule 校验「指定域名仅允许纯数字本地部分」规则（契约 phase4 功能 1）。
//
// 语义（boss 最终确认，方向不可反）：
//   - 仅当 common.EmailNumericOnlyRestrictionEnabled 为 true 且邮箱域名命中
//     common.EmailNumericOnlyDomainWhitelist（大小写不敏感）时生效；
//   - 生效时 @ 之前的本地部分必须「每个字符都是数字」（unicode.IsDigit，覆盖全角数字等
//     Unicode 数字）；只要包含字母或任何非数字字符即拒绝；
//   - 纯数字不限长度，放行。
//
// 返回 nil 表示通过。未启用、域名不命中、本地部分为纯数字时均返回 nil。
// 邮箱格式非法（没有 @ 或多个 @）时返回 nil：格式校验由既有 validator 负责，本函数不越权。
func ValidateEmailNumericOnlyRule(email string) error {
	if !common.EmailNumericOnlyRestrictionEnabled {
		return nil
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		// 无 @ / 多个 @ 属于格式问题，交给既有 validator 处理。
		return nil
	}
	localPart := parts[0]
	domain := strings.ToLower(strings.TrimSpace(parts[1]))
	if !emailNumericOnlyDomainMatched(domain) {
		return nil
	}
	for _, char := range localPart {
		if !unicode.IsDigit(char) {
			return fmt.Errorf("该邮箱域名（%s）仅允许纯数字账号，请使用纯数字邮箱地址", domain)
		}
	}
	return nil
}

// emailNumericOnlyDomainMatched 判断邮箱域名是否命中「仅允许纯数字本地部分」的域名列表。
// 比较时统一转小写并去除首尾空白，保证后台配置与用户输入的域名大小写不敏感。
func emailNumericOnlyDomainMatched(domain string) bool {
	for _, allowed := range common.EmailNumericOnlyDomainWhitelist {
		if strings.ToLower(strings.TrimSpace(allowed)) == domain {
			return true
		}
	}
	return false
}

// parseEmailNumericOnlyDomainWhitelist 解析后台保存的域名列表（逗号分隔）：
// 去首尾空白、丢弃空项、统一小写，避免用空串把「启用但无域名」的空列表保存进去。
func parseEmailNumericOnlyDomainWhitelist(value string) []string {
	parts := strings.Split(value, ",")
	domains := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		domains = append(domains, part)
	}
	return domains
}
