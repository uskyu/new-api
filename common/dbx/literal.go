package dbx

func BoolTrueLiteral() string {
	if IsPostgres() {
		return "true"
	}
	return "1"
}

func BoolFalseLiteral() string {
	if IsPostgres() {
		return "false"
	}
	return "0"
}
