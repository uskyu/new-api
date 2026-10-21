package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// setupEmailNumericOnlyRuleTest 设置全局配置并在用例结束后恢复，避免污染其他测试。
func setupEmailNumericOnlyRuleTest(t *testing.T, enabled bool, domains []string) {
	t.Helper()
	prevEnabled := common.EmailNumericOnlyRestrictionEnabled
	prevDomains := common.EmailNumericOnlyDomainWhitelist
	common.EmailNumericOnlyRestrictionEnabled = enabled
	common.EmailNumericOnlyDomainWhitelist = append([]string(nil), domains...)
	t.Cleanup(func() {
		common.EmailNumericOnlyRestrictionEnabled = prevEnabled
		common.EmailNumericOnlyDomainWhitelist = prevDomains
	})
}

// TestValidateEmailNumericOnlyRuleDefaultDisabled 默认关闭时任何邮箱都放行（升级行为不变）。
func TestValidateEmailNumericOnlyRuleDefaultDisabled(t *testing.T) {
	setupEmailNumericOnlyRuleTest(t, false, []string{"qq.com"})
	require.NoError(t, ValidateEmailNumericOnlyRule("abc123456@qq.com"))
	require.NoError(t, ValidateEmailNumericOnlyRule("qq12345@qq.com"))
	require.NoError(t, ValidateEmailNumericOnlyRule("12345678@qq.com"))
}

// TestValidateEmailNumericOnlyRuleNumericLocalPartAllowed 启用后纯数字本地部分放行（不限长度）。
func TestValidateEmailNumericOnlyRuleNumericLocalPartAllowed(t *testing.T) {
	setupEmailNumericOnlyRuleTest(t, true, []string{"qq.com"})
	require.NoError(t, ValidateEmailNumericOnlyRule("12345678@qq.com"))
	require.NoError(t, ValidateEmailNumericOnlyRule("0@qq.com"))
	require.NoError(t, ValidateEmailNumericOnlyRule("1@qq.com"))
	require.NoError(t, ValidateEmailNumericOnlyRule(strings.Repeat("9", 200)+"@qq.com"))
}

// TestValidateEmailNumericOnlyRuleRejectsNonNumericLocalPart 启用后含字母/符号的本地部分一律拒绝。
func TestValidateEmailNumericOnlyRuleRejectsNonNumericLocalPart(t *testing.T) {
	setupEmailNumericOnlyRuleTest(t, true, []string{"qq.com"})
	rejected := []string{
		"abc123456@qq.com",
		"qq12345@qq.com",
		"12345678abc@qq.com",
		"a@qq.com",
		"123_456@qq.com",
		"123-456@qq.com",
		"123.456@qq.com",
		"123 456@qq.com",
	}
	for _, email := range rejected {
		err := ValidateEmailNumericOnlyRule(email)
		require.Errorf(t, err, "email %q 应被拒绝", email)
		require.Contains(t, err.Error(), "纯数字", "错误信息需明确告知仅允许纯数字账号")
	}
}

// TestValidateEmailNumericOnlyRuleIgnoresOtherDomains 其他域名完全不检查。
func TestValidateEmailNumericOnlyRuleIgnoresOtherDomains(t *testing.T) {
	setupEmailNumericOnlyRuleTest(t, true, []string{"qq.com"})
	require.NoError(t, ValidateEmailNumericOnlyRule("abc123456@gmail.com"))
	require.NoError(t, ValidateEmailNumericOnlyRule("abc123456@example.com"))
	require.NoError(t, ValidateEmailNumericOnlyRule("abc123456@163.com"))
}

// TestValidateEmailNumericOnlyRuleDomainCaseInsensitive 域名大小写不敏感（配置与输入两侧都归一）。
func TestValidateEmailNumericOnlyRuleDomainCaseInsensitive(t *testing.T) {
	setupEmailNumericOnlyRuleTest(t, true, []string{"QQ.COM"})
	require.NoError(t, ValidateEmailNumericOnlyRule("12345678@qq.com"))
	require.NoError(t, ValidateEmailNumericOnlyRule("12345678@QQ.COM"))
	require.NoError(t, ValidateEmailNumericOnlyRule("12345678@Qq.CoM"))
	require.Error(t, ValidateEmailNumericOnlyRule("abc123456@QQ.COM"))
}

// TestValidateEmailNumericOnlyRuleFullWidthDigits 全角数字按 Unicode 数字放行；全角字母拒绝。
func TestValidateEmailNumericOnlyRuleFullWidthDigits(t *testing.T) {
	setupEmailNumericOnlyRuleTest(t, true, []string{"qq.com"})
	require.NoError(t, ValidateEmailNumericOnlyRule("１２３４５６７８@qq.com"))
	require.NoError(t, ValidateEmailNumericOnlyRule("１２３456@qq.com"))
	require.Error(t, ValidateEmailNumericOnlyRule("ａｂｃ１２３@qq.com"))
}

// TestValidateEmailNumericOnlyRuleInvalidFormatIgnored 邮箱格式非法时不越权（返回 nil，交给既有 validator）。
func TestValidateEmailNumericOnlyRuleInvalidFormatIgnored(t *testing.T) {
	setupEmailNumericOnlyRuleTest(t, true, []string{"qq.com"})
	for _, email := range []string{"", "not-an-email", "a@b@qq.com", "abc@", "@qq.com"} {
		require.NoErrorf(t, ValidateEmailNumericOnlyRule(email), "非法格式 %q 不应由本规则处理", email)
	}
}

// TestValidateEmailNumericOnlyRuleMultipleDomains 多域名列表：只对列表内域名生效。
func TestValidateEmailNumericOnlyRuleMultipleDomains(t *testing.T) {
	setupEmailNumericOnlyRuleTest(t, true, []string{"gmail.com", "qq.com"})
	require.NoError(t, ValidateEmailNumericOnlyRule("12345678@gmail.com"))
	require.NoError(t, ValidateEmailNumericOnlyRule("12345678@qq.com"))
	require.Error(t, ValidateEmailNumericOnlyRule("abc@gmail.com"))
	require.Error(t, ValidateEmailNumericOnlyRule("abc@qq.com"))
	// 不在列表内的域名含字母也放行。
	require.NoError(t, ValidateEmailNumericOnlyRule("abc123@163.com"))
	// 列表中的域名即使为空项/大小写混写也不影响匹配。
	setupEmailNumericOnlyRuleTest(t, true, []string{"  GMAIL.COM  ", "", "qq.com"})
	require.Error(t, ValidateEmailNumericOnlyRule("abc@gmail.com"))
	require.NoError(t, ValidateEmailNumericOnlyRule("123@gmail.com"))
}

// TestParseEmailNumericOnlyDomainWhitelist 配置解析：去空白、去空项、转小写。
func TestParseEmailNumericOnlyDomainWhitelist(t *testing.T) {
	require.Equal(t, []string{"qq.com", "gmail.com"}, parseEmailNumericOnlyDomainWhitelist(" QQ.COM , ,Gmail.com,  "))
	require.Empty(t, parseEmailNumericOnlyDomainWhitelist(""))
	require.Empty(t, parseEmailNumericOnlyDomainWhitelist("  ,  "))
	require.Equal(t, []string{"qq.com"}, parseEmailNumericOnlyDomainWhitelist("qq.com"))
}

// TestEmailNumericOnlyOptionWiring 配置项接入：updateOptionMap 生效 + InitOptionMap 预填默认值。
func TestEmailNumericOnlyOptionWiring(t *testing.T) {
	prevEnabled := common.EmailNumericOnlyRestrictionEnabled
	prevDomains := common.EmailNumericOnlyDomainWhitelist
	prevOptionMap := common.OptionMap
	t.Cleanup(func() {
		common.EmailNumericOnlyRestrictionEnabled = prevEnabled
		common.EmailNumericOnlyDomainWhitelist = prevDomains
		common.OptionMap = prevOptionMap
	})
	// TestMain 不一定初始化 OptionMap，这里先建立默认表（等价于服务启动时的 InitOptionMap）。
	InitOptionMap()

	require.NoError(t, updateOptionMap("EmailNumericOnlyRestrictionEnabled", "true"))
	require.True(t, common.EmailNumericOnlyRestrictionEnabled)
	require.Equal(t, "true", common.OptionMap["EmailNumericOnlyRestrictionEnabled"])

	require.NoError(t, updateOptionMap("EmailNumericOnlyDomainWhitelist", " QQ.COM , ,Gmail.com,  "))
	require.Equal(t, []string{"qq.com", "gmail.com"}, common.EmailNumericOnlyDomainWhitelist)
	require.Equal(t, " QQ.COM , ,Gmail.com,  ", common.OptionMap["EmailNumericOnlyDomainWhitelist"])

	// InitOptionMap 用当前全局变量重新预填默认值。
	common.EmailNumericOnlyRestrictionEnabled = true
	common.EmailNumericOnlyDomainWhitelist = []string{"qq.com", "163.com"}
	InitOptionMap()
	require.Equal(t, "true", common.OptionMap["EmailNumericOnlyRestrictionEnabled"])
	require.Equal(t, "qq.com,163.com", common.OptionMap["EmailNumericOnlyDomainWhitelist"])
}
