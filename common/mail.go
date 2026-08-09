package common

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	pb "server/proto/pb"
)

// NormalizeSystemMailText 校验系统邮件文本并返回实际持久化的规范值。
func NormalizeSystemMailText(title string, content string) (string, string, error) {
	if !utf8.ValidString(title) || !utf8.ValidString(content) {
		return "", "", fmt.Errorf("mail text is not valid UTF-8")
	}

	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > int(pb.Constants_Constants_Mail_Title_Max_Length) {
		return "", "", fmt.Errorf("mail title length is invalid")
	}
	for _, r := range title {
		if unicode.IsControl(r) {
			return "", "", fmt.Errorf("mail title contains control character")
		}
	}

	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	if strings.TrimSpace(content) == "" || utf8.RuneCountInString(content) > int(pb.Constants_Constants_Mail_Content_Max_Length) {
		return "", "", fmt.Errorf("mail content length is invalid")
	}
	for _, r := range content {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return "", "", fmt.Errorf("mail content contains control character")
		}
	}

	return title, content, nil
}
