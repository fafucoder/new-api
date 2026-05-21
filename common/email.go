package common

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime/multipart"
	"net/smtp"
	"net/textproto"
	"slices"
	"strings"
	"time"
)

func generateMessageID() (string, error) {
	split := strings.Split(SMTPFrom, "@")
	if len(split) < 2 {
		return "", fmt.Errorf("invalid SMTP account")
	}
	domain := strings.Split(SMTPFrom, "@")[1]
	return fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), GetRandomString(12), domain), nil
}

func shouldUseSMTPLoginAuth() bool {
	if SMTPForceAuthLogin {
		return true
	}
	return isOutlookServer(SMTPAccount) || slices.Contains(EmailLoginAuthServerList, SMTPServer)
}

func getSMTPAuth() smtp.Auth {
	if shouldUseSMTPLoginAuth() {
		return LoginAuth(SMTPAccount, SMTPToken)
	}
	return smtp.PlainAuth("", SMTPAccount, SMTPToken, SMTPServer)
}

func SendEmail(subject string, receiver string, content string) error {
	if SMTPFrom == "" { // for compatibility
		SMTPFrom = SMTPAccount
	}
	id, err2 := generateMessageID()
	if err2 != nil {
		return err2
	}
	if SMTPServer == "" && SMTPAccount == "" {
		return fmt.Errorf("SMTP 服务器未配置")
	}
	encodedSubject := fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(subject)))
	mail := []byte(fmt.Sprintf("To: %s\r\n"+
		"From: %s <%s>\r\n"+
		"Subject: %s\r\n"+
		"Date: %s\r\n"+
		"Message-ID: %s\r\n"+ // 添加 Message-ID 头
		"Content-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n",
		receiver, SystemName, SMTPFrom, encodedSubject, time.Now().Format(time.RFC1123Z), id, content))
	auth := getSMTPAuth()
	addr := fmt.Sprintf("%s:%d", SMTPServer, SMTPPort)
	to := strings.Split(receiver, ";")
	var err error
	if SMTPPort == 465 || SMTPSSLEnabled {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         SMTPServer,
		}
		conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", SMTPServer, SMTPPort), tlsConfig)
		if err != nil {
			return err
		}
		client, err := smtp.NewClient(conn, SMTPServer)
		if err != nil {
			return err
		}
		defer client.Close()
		if err = client.Auth(auth); err != nil {
			return err
		}
		if err = client.Mail(SMTPFrom); err != nil {
			return err
		}
		receiverEmails := strings.Split(receiver, ";")
		for _, receiver := range receiverEmails {
			if err = client.Rcpt(receiver); err != nil {
				return err
			}
		}
		w, err := client.Data()
		if err != nil {
			return err
		}
		_, err = w.Write(mail)
		if err != nil {
			return err
		}
		err = w.Close()
		if err != nil {
			return err
		}
	} else {
		err = smtp.SendMail(addr, auth, SMTPFrom, to, mail)
	}
	if err != nil {
		SysError(fmt.Sprintf("failed to send email to %s: %v", receiver, err))
	}
	return err
}

// SendEmailWithAttachment 发送带附件的邮件。
func SendEmailWithAttachment(subject, receiver, content, attachmentName string, attachmentData []byte) error {
	if SMTPFrom == "" {
		SMTPFrom = SMTPAccount
	}
	if SMTPServer == "" && SMTPAccount == "" {
		return fmt.Errorf("SMTP 服务器未配置")
	}
	id, err := generateMessageID()
	if err != nil {
		return err
	}
	encodedSubject := fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(subject)))

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	header := fmt.Sprintf("To: %s\r\nFrom: %s <%s>\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=%s\r\n\r\n",
		receiver, SystemName, SMTPFrom, encodedSubject,
		time.Now().Format(time.RFC1123Z), id, w.Boundary())

	// HTML body part
	hw, _ := w.CreatePart(textproto.MIMEHeader{
		"Content-Type": {"text/html; charset=UTF-8"},
	})
	fmt.Fprint(hw, content)

	// Attachment part
	encodedName := fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(attachmentName)))
	aw, _ := w.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {"application/octet-stream"},
		"Content-Transfer-Encoding": {"base64"},
		"Content-Disposition":       {fmt.Sprintf(`attachment; filename="%s"`, encodedName)},
	})
	enc := base64.NewEncoder(base64.StdEncoding, aw)
	enc.Write(attachmentData)
	enc.Close()
	w.Close()

	mail := append([]byte(header), buf.Bytes()...)
	auth := getSMTPAuth()
	addr := fmt.Sprintf("%s:%d", SMTPServer, SMTPPort)
	to := strings.Split(receiver, ";")

	if SMTPPort == 465 || SMTPSSLEnabled {
		tlsConfig := &tls.Config{InsecureSkipVerify: true, ServerName: SMTPServer}
		conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", SMTPServer, SMTPPort), tlsConfig)
		if err != nil {
			return err
		}
		client, err := smtp.NewClient(conn, SMTPServer)
		if err != nil {
			return err
		}
		defer client.Close()
		if err = client.Auth(auth); err != nil {
			return err
		}
		if err = client.Mail(SMTPFrom); err != nil {
			return err
		}
		for _, r := range to {
			if err = client.Rcpt(r); err != nil {
				return err
			}
		}
		dw, err := client.Data()
		if err != nil {
			return err
		}
		if _, err = dw.Write(mail); err != nil {
			return err
		}
		err = dw.Close()
		if err != nil {
			SysError(fmt.Sprintf("failed to send email with attachment to %s: %v", receiver, err))
		}
		return err
	}
	err = smtp.SendMail(addr, auth, SMTPFrom, to, mail)
	if err != nil {
		SysError(fmt.Sprintf("failed to send email with attachment to %s: %v", receiver, err))
	}
	return err
}
