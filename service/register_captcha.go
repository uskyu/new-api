package service

// 注册图形验证码。后台开关与类型由 common.RegisterCaptchaEnabled / common.RegisterCaptchaKind 控制，
// 挑战池与签到图形验证码隔离，避免两类场景互相消费验证码。

var registerCaptcha = newImageCaptchaState()

// GenerateRegisterCaptcha 生成注册图形验证码挑战，kind 支持 math / digit。
func GenerateRegisterCaptcha(kind string) (id string, imageBase64 string, err error) {
	return registerCaptcha.generate(kind)
}

// VerifyRegisterCaptcha 校验并消费一次性注册验证码。
func VerifyRegisterCaptcha(id string, answer string) error {
	return registerCaptcha.verify(id, answer)
}
