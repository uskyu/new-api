package service

// 签到图形验证码。生成与校验逻辑和注册图形验证码共用 image_captcha.go，
// 但两者使用独立的挑战池，互不消费对方的验证码。

var checkinCaptcha = newImageCaptchaState()

// GenerateCheckinCaptcha 生成签到图形验证码挑战，kind 支持 math / digit。
func GenerateCheckinCaptcha(kind string) (id string, imageBase64 string, err error) {
	return checkinCaptcha.generate(kind)
}

// VerifyCheckinCaptcha 校验并消费一次性签到验证码。
func VerifyCheckinCaptcha(id string, answer string) error {
	return checkinCaptcha.verify(id, answer)
}
