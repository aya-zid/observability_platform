package service

import (
	"crypto/tls"
	"fmt"
	"html"
	"log"
	"net/smtp"
	"strings"
	"time"
)

type EmailSender struct {
	fromName    string
	fromAddress string
	smtpHost    string
	smtpPort    string
	smtpUser    string
	smtpPass    string
	platformURL string
	enabled     bool
}

func NewEmailSender(smtpHost, smtpPort, smtpUser, smtpPass, fromAddress, fromName, platformURL string) *EmailSender {
	enabled := smtpHost != "" && smtpUser != "" && smtpPass != "" && fromAddress != ""
	if !enabled {
		log.Println("[email] SMTP not configured — emails will be logged only")
	}
	return &EmailSender{
		fromName:    fromName,
		fromAddress: fromAddress,
		smtpHost:    smtpHost,
		smtpPort:    smtpPort,
		smtpUser:    smtpUser,
		smtpPass:    smtpPass,
		platformURL: strings.TrimRight(platformURL, "/"),
		enabled:     enabled,
	}
}

func (s *EmailSender) IsEnabled() bool {
	return s.enabled
}

func (s *EmailSender) FromAddress() string { return s.fromAddress }

func (s *EmailSender) FromName() string { return s.fromName }

func (s *EmailSender) SMTPHost() string { return s.smtpHost }

func (s *EmailSender) Send(to, subject, htmlBody string) error {
	if !s.enabled {
		log.Printf("[email] [SKIP] To: %s | Subject: %s", to, subject)
		return nil
	}

	msg := s.buildMessage(to, subject, htmlBody)
	addr := fmt.Sprintf("%s:%s", s.smtpHost, s.smtpPort)
	auth := smtp.PlainAuth("", s.smtpUser, s.smtpPass, s.smtpHost)

	if s.smtpPort == "465" {
		tlsCfg := &tls.Config{ServerName: s.smtpHost}
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return fmt.Errorf("tls dial: %w", err)
		}
		client, err := smtp.NewClient(conn, s.smtpHost)
		if err != nil {
			return fmt.Errorf("smtp client: %w", err)
		}
		defer client.Close()
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
		if err = client.Mail(s.fromAddress); err != nil {
			return fmt.Errorf("smtp mail: %w", err)
		}
		if err = client.Rcpt(to); err != nil {
			return fmt.Errorf("smtp rcpt: %w", err)
		}
		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("smtp data: %w", err)
		}
		_, err = w.Write([]byte(msg))
		if err != nil {
			return fmt.Errorf("smtp write: %w", err)
		}
		return w.Close()
	}

	return smtp.SendMail(addr, auth, s.fromAddress, []string{to}, []byte(msg))
}

func (s *EmailSender) buildMessage(to, subject, htmlBody string) string {
	headers := make(map[string]string)
	headers["From"] = fmt.Sprintf("%s <%s>", s.fromName, s.fromAddress)
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=\"UTF-8\""
	headers["Content-Transfer-Encoding"] = "quoted-printable"
	headers["Date"] = time.Now().Format(time.RFC1123Z)

	var sb strings.Builder
	for k, v := range headers {
		sb.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	sb.WriteString("\r\n")
	sb.WriteString(htmlBody)
	return sb.String()
}

// ── HTML Templates ──────────────────────────────────────────────

func (s *EmailSender) InviteTeammateHTML(inviterName, companyName, inviteeEmail, role, inviteURL string) string {
	roleDisplay := strings.Title(role)
	inviterName = html.EscapeString(inviterName)
	companyName = html.EscapeString(companyName)
	inviteeEmail = html.EscapeString(inviteeEmail)
	roleDisplay = html.EscapeString(roleDisplay)
	inviteURL = html.EscapeString(inviteURL)
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin:0;padding:0;background-color:#f1f5f9;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Arial,sans-serif;color:#172033">
  <div style="display:none;max-height:0;overflow:hidden;opacity:0;color:transparent">You have been invited to join %s on MediaNet Observability.</div>
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:#f1f5f9;padding:40px 16px">
    <tr>
      <td align="center">
        <table role="presentation" width="600" cellpadding="0" cellspacing="0" style="max-width:600px;width:100%%">
          <tr>
            <td style="padding:0 0 22px;text-align:center">
              <table role="presentation" cellpadding="0" cellspacing="0" style="margin:0 auto"><tr>
                <td style="width:34px;height:34px;border-radius:9px;background:#0b6e99;color:#ffffff;font-size:18px;font-weight:800;text-align:center;line-height:34px">M</td>
                <td style="padding-left:10px;font-size:19px;font-weight:750;letter-spacing:-0.3px;color:#14213d">MediaNet <span style="color:#0b6e99">Observability</span></td>
              </tr></table>
            </td>
          </tr>
          <tr>
            <td style="background:#ffffff;border:1px solid #dce4ee;border-radius:16px;padding:40px 40px 36px;box-shadow:0 10px 30px rgba(15,23,42,0.08)">
              <div style="display:inline-block;margin:0 0 16px;padding:6px 10px;border-radius:999px;background:#e8f5fa;color:#086287;font-size:12px;font-weight:700;letter-spacing:.5px;text-transform:uppercase">Team invitation</div>
              <h1 style="margin:0 0 12px;font-size:28px;line-height:1.22;letter-spacing:-.5px;color:#14213d;font-weight:750">Build better visibility, together.</h1>
              <p style="margin:0 0 26px;font-size:16px;color:#526174;line-height:1.6">
                <strong style="color:#172033">%s</strong> invited you to join <strong style="color:#172033">%s</strong> in MediaNet Observability.
              </p>
              <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="margin:0 0 28px;border:1px solid #e1e8f0;border-radius:10px;background:#f8fafc">
                <tr>
                  <td style="padding:14px 16px 8px;font-size:12px;font-weight:700;letter-spacing:.4px;color:#718096;text-transform:uppercase">Workspace</td>
                  <td style="padding:14px 16px 8px;font-size:12px;font-weight:700;letter-spacing:.4px;color:#718096;text-transform:uppercase">Role</td>
                </tr>
                <tr>
                  <td style="padding:0 16px 15px;font-size:15px;font-weight:650;color:#172033">%s</td>
                  <td style="padding:0 16px 15px;font-size:15px;font-weight:650;color:#172033">%s</td>
                </tr>
              </table>
              <a href="%s" style="display:inline-block;background:#0b6e99;color:#ffffff;text-decoration:none;font-size:15px;font-weight:700;padding:14px 24px;border-radius:9px">Accept invitation &rarr;</a>
              <p style="margin:26px 0 0;padding-top:20px;border-top:1px solid #e8edf3;font-size:13px;color:#718096;line-height:1.55">
                This invitation was sent to %s and expires in <strong style="color:#526174">72 hours</strong>. If you were not expecting it, you can safely ignore this email.
              </p>
            </td>
          </tr>
          <tr>
            <td style="padding:24px 16px 0;text-align:center;font-size:12px;color:#718096;line-height:1.6">
              <p style="margin:0 0 8px;font-weight:650;color:#526174">MediaNet Observability</p>
              <p style="margin:0">Can’t use the button? Copy this link into your browser:<br><a href="%s" style="color:#0b6e99;word-break:break-all">%s</a></p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, companyName, inviterName, companyName, companyName, roleDisplay, inviteURL, inviteeEmail, inviteURL, inviteURL)
}

func (s *EmailSender) PasswordResetHTML(resetURL string) string {
	resetURL = html.EscapeString(resetURL)
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin:0;padding:0;background-color:#f1f5f9;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Arial,sans-serif;color:#172033">
  <div style="display:none;max-height:0;overflow:hidden;opacity:0;color:transparent">Reset your MediaNet Observability password.</div>
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:#f1f5f9;padding:40px 16px">
    <tr>
      <td align="center">
        <table role="presentation" width="600" cellpadding="0" cellspacing="0" style="max-width:600px;width:100%%">
          <tr>
            <td style="padding:0 0 22px;text-align:center">
              <table role="presentation" cellpadding="0" cellspacing="0" style="margin:0 auto"><tr>
                <td style="width:34px;height:34px;border-radius:9px;background:#0b6e99;color:#ffffff;font-size:18px;font-weight:800;text-align:center;line-height:34px">M</td>
                <td style="padding-left:10px;font-size:19px;font-weight:750;letter-spacing:-0.3px;color:#14213d">MediaNet <span style="color:#0b6e99">Observability</span></td>
              </tr></table>
            </td>
          </tr>
          <tr>
            <td style="background:#ffffff;border:1px solid #dce4ee;border-radius:16px;padding:40px 40px 36px;box-shadow:0 10px 30px rgba(15,23,42,0.08)">
              <div style="display:inline-block;margin:0 0 16px;padding:6px 10px;border-radius:999px;background:#e8f5fa;color:#086287;font-size:12px;font-weight:700;letter-spacing:.5px;text-transform:uppercase">Account security</div>
              <h1 style="margin:0 0 12px;font-size:28px;line-height:1.22;letter-spacing:-.5px;color:#14213d;font-weight:750">Reset your password</h1>
              <p style="margin:0 0 26px;font-size:16px;color:#526174;line-height:1.6">
                A password-reset request was received for your MediaNet Observability account. Use the secure link below to choose a new password.
              </p>
              <a href="%s" style="display:inline-block;background:#0b6e99;color:#ffffff;text-decoration:none;font-size:15px;font-weight:700;padding:14px 24px;border-radius:9px">Reset password &rarr;</a>
              <p style="margin:26px 0 0;padding:16px;border-radius:9px;background:#f8fafc;font-size:13px;color:#526174;line-height:1.55">
                <strong style="color:#172033">For your security:</strong> this link expires in one hour and can be used once. If you did not request a reset, no action is needed.
              </p>
            </td>
          </tr>
          <tr>
            <td style="padding:24px 16px 0;text-align:center;font-size:12px;color:#718096;line-height:1.6">
              <p style="margin:0 0 8px;font-weight:650;color:#526174">MediaNet Observability</p>
              <p style="margin:0">Can’t use the button? Copy this link into your browser:<br><a href="%s" style="color:#0b6e99;word-break:break-all">%s</a></p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, resetURL, resetURL, resetURL)
}

func (s *EmailSender) AlertFiredHTML(ruleName, serviceType, severity, hostname, serviceName, domain, description string, metricValue, thresholdValue *float64, unit string, dashboardURL string) string {
	severityColor := "#f59e0b"
	severityLabel := "Warning"
	if severity == "critical" {
		severityColor = "#ef4444"
		severityLabel = "Critical"
	} else if severity == "info" {
		severityColor = "#3b82f6"
		severityLabel = "Info"
	}
	serviceLabel := strings.Title(serviceType)
	timeStr := time.Now().Format("Jan 2, 2006 15:04 UTC")

	// Build extra detail rows
	var detailRows string
	if hostname != "" {
		detailRows += fmt.Sprintf(`                <tr>
                  <td style="padding:10px 0;border-top:1px solid #e2e8f0;font-size:14px;color:#64748b">Host</td>
                  <td style="padding:10px 0;border-top:1px solid #e2e8f0;font-size:14px;color:#1e293b;font-weight:500;text-align:right">%s</td>
                </tr>`, hostname)
	}
	if serviceName != "" {
		detailRows += fmt.Sprintf(`                <tr>
                  <td style="padding:10px 0;border-top:1px solid #e2e8f0;font-size:14px;color:#64748b">Service</td>
                  <td style="padding:10px 0;border-top:1px solid #e2e8f0;font-size:14px;color:#1e293b;font-weight:500;text-align:right">%s</td>
                </tr>`, serviceName)
	}
	if domain != "" {
		detailRows += fmt.Sprintf(`                <tr>
                  <td style="padding:10px 0;border-top:1px solid #e2e8f0;font-size:14px;color:#64748b">Domain</td>
                  <td style="padding:10px 0;border-top:1px solid #e2e8f0;font-size:14px;color:#1e293b;font-weight:500;text-align:right">%s</td>
                </tr>`, domain)
	}
	if metricValue != nil {
		mv := formatFloat(*metricValue)
		if unit != "" {
			mv += " " + unit
		}
		detailRows += fmt.Sprintf(`                <tr>
                  <td style="padding:10px 0;border-top:1px solid #e2e8f0;font-size:14px;color:#64748b">Current Value</td>
                  <td style="padding:10px 0;border-top:1px solid #e2e8f0;font-size:14px;color:#ef4444;font-weight:600;text-align:right">%s</td>
                </tr>`, mv)
	}
	if thresholdValue != nil {
		tv := formatFloat(*thresholdValue)
		if unit != "" {
			tv += " " + unit
		}
		detailRows += fmt.Sprintf(`                <tr>
                  <td style="padding:10px 0;border-top:1px solid #e2e8f0;font-size:14px;color:#64748b">Threshold</td>
                  <td style="padding:10px 0;border-top:1px solid #e2e8f0;font-size:14px;color:#1e293b;font-weight:500;text-align:right">%s</td>
                </tr>`, tv)
	}
	detailRows += fmt.Sprintf(`                <tr>
                  <td style="padding:10px 0;border-top:1px solid #e2e8f0;font-size:14px;color:#64748b">Service Type</td>
                  <td style="padding:10px 0;border-top:1px solid #e2e8f0;font-size:14px;color:#1e293b;font-weight:500;text-align:right">%s</td>
                </tr>`, serviceLabel)
	detailRows += fmt.Sprintf(`                <tr>
                  <td style="padding:10px 0;border-top:1px solid #e2e8f0;font-size:14px;color:#64748b">Time</td>
                  <td style="padding:10px 0;border-top:1px solid #e2e8f0;font-size:14px;color:#1e293b;text-align:right">%s</td>
                </tr>`, timeStr)

	severityRow := fmt.Sprintf(`                <tr>
                  <td style="padding:10px 0;border-top:1px solid #e2e8f0;font-size:14px;color:#64748b">Severity</td>
                  <td style="padding:10px 0;border-top:1px solid #e2e8f0;font-size:14px;color:%s;font-weight:600;text-align:right">%s</td>
                </tr>`, severityColor, severityLabel)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin:0;padding:0;background-color:#f4f5f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">
  <table width="100%%" cellpadding="0" cellspacing="0" style="background-color:#f4f5f7;padding:40px 16px">
    <tr>
      <td align="center">
        <table width="560" cellpadding="0" cellspacing="0" style="max-width:560px;width:100%%">
          <tr>
            <td style="padding-bottom:24px;text-align:center">
              <span style="font-size:22px;font-weight:700;color:#1e293b">Observability Platform</span>
            </td>
          </tr>
          <tr>
            <td style="background-color:#ffffff;border-radius:12px;padding:40px 32px;box-shadow:0 1px 3px rgba(0,0,0,0.08)">
              <div style="display:inline-block;background-color:%[1]s15;color:%[1]s;font-size:12px;font-weight:600;padding:4px 12px;border-radius:20px;margin-bottom:12px;text-transform:uppercase">%[2]s</div>
              <h1 style="margin:0 0 8px;font-size:20px;color:#1e293b;font-weight:600">%[3]s</h1>
              <p style="margin:0 0 24px;font-size:14px;color:#64748b;line-height:1.5">%[4]s</p>
              <table cellpadding="0" cellspacing="0" style="margin-bottom:24px;width:100%%">
                %s
                %s
              </table>
              <a href="%s" style="display:inline-block;background-color:#2563eb;color:#ffffff;text-decoration:none;font-size:15px;font-weight:600;padding:14px 32px;border-radius:8px">View in Dashboard</a>
            </td>
          </tr>
          <tr>
            <td style="padding:24px 16px 0;text-align:center;font-size:12px;color:#94a3b8">
              <p style="margin:0">This is an automated alert from your Observability Platform monitoring system.</p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, severityColor, severityLabel, ruleName, description, detailRows, severityRow, dashboardURL)
}

func (s *EmailSender) TestEmailHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin:0;padding:0;background-color:#f4f5f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">
  <table width="100%" cellpadding="0" cellspacing="0" style="background-color:#f4f5f7;padding:40px 16px">
    <tr>
      <td align="center">
        <table width="560" cellpadding="0" cellspacing="0" style="max-width:560px;width:100%">
          <tr>
            <td style="padding-bottom:24px;text-align:center">
              <span style="font-size:22px;font-weight:700;color:#1e293b">Observability Platform</span>
            </td>
          </tr>
          <tr>
            <td style="background-color:#ffffff;border-radius:12px;padding:40px 32px;box-shadow:0 1px 3px rgba(0,0,0,0.08)">
              <div style="width:48px;height:48px;background-color:#22c55e15;border-radius:50%;display:flex;align-items:center;justify-content:center;margin-bottom:16px">
                <span style="font-size:24px">✅</span>
              </div>
              <h1 style="margin:0 0 8px;font-size:20px;color:#1e293b;font-weight:600">SMTP Configuration Verified</h1>
              <p style="margin:0;font-size:14px;color:#64748b;line-height:1.5">Your email settings are working correctly. You will now receive team invitations and alert notifications at this address.</p>
            </td>
          </tr>
          <tr>
            <td style="padding:24px 16px 0;text-align:center;font-size:12px;color:#94a3b8">
              <p style="margin:0">Observability Platform &middot; Real-time infrastructure monitoring</p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`
}
