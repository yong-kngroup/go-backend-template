package verification

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	domainVerification "github.com/freeDog-wy/go-backend-template/internal/domain/verification"
	"github.com/freeDog-wy/go-backend-template/pkg/email"
	"github.com/freeDog-wy/go-backend-template/pkg/logger"
)

// EmailVerificationConsumer 消费验证相关事件并发送邮件。
type EmailVerificationConsumer struct {
	emailSender email.Sender
	siteBaseURL string
	logger      logger.Logger
}

func NewConsumer(emailSender email.Sender, siteBaseURL string, logger logger.Logger) *EmailVerificationConsumer {
	return &EmailVerificationConsumer{
		emailSender: emailSender,
		siteBaseURL: strings.TrimRight(siteBaseURL, "/"),
		logger:      logger,
	}
}

// OnEmailVerificationRequested 消费 user.email_verification_requested 事件——发送验证邮件。
func (c *EmailVerificationConsumer) OnEmailVerificationRequested(_ context.Context, evt domainVerification.EmailVerificationRequested) error {
	if c.emailSender == nil {
		return nil
	}

	link := "/verify-email?token=" + url.QueryEscape(evt.Token)
	if c.siteBaseURL != "" {
		link = c.siteBaseURL + link
	}
	body := fmt.Sprintf("欢迎注册，请点击以下链接验证您的邮箱：\n%s", link)

	if err := c.emailSender.Send(evt.Email, "邮箱验证", body); err != nil {
		if c.logger != nil {
			c.logger.Error("verification email failed", "email", evt.Email, "error", err)
		}
		return err
	}

	if c.logger != nil {
		c.logger.Info("verification email sent", "email", evt.Email)
	}
	return nil
}

func (c *EmailVerificationConsumer) OnPasswordResetRequested(_ context.Context, evt domainVerification.PasswordResetRequested) error {
	if c.emailSender == nil {
		return nil
	}

	link := "/reset-password?token=" + url.QueryEscape(evt.Token)
	if c.siteBaseURL != "" {
		link = c.siteBaseURL + link
	}
	body := fmt.Sprintf("您正在重置密码，请点击以下链接继续操作：\n%s", link)

	if err := c.emailSender.Send(evt.Email, "重置密码", body); err != nil {
		if c.logger != nil {
			c.logger.Error("password reset email failed", "email", evt.Email, "error", err)
		}
		return err
	}

	if c.logger != nil {
		c.logger.Info("password reset email sent", "email", evt.Email)
	}
	return nil
}
