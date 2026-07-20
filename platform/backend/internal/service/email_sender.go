package service

import (
	"crypto/tls"
	"fmt"
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
          <!-- Logo area -->
          <tr>
            <td style="padding-bottom:24px;text-align:center">
              <span style="font-size:22px;font-weight:700;color:#1e293b">Observability Platform</span>
            </td>
          </tr>
          <!-- Card -->
          <tr>
            <td style="background-color:#ffffff;border-radius:12px;padding:40px 32px;box-shadow:0 1px 3px rgba(0,0,0,0.08)">
              <h1 style="margin:0 0 8px;font-size:22px;color:#1e293b;font-weight:600">You're invited to join %s</h1>
              <p style="margin:0 0 24px;font-size:15px;color:#64748b;line-height:1.5">
                <strong style="color:#334155">%s</strong> has invited you to join <strong>%s</strong> as a <strong>%s</strong>.
              </p>
              <table cellpadding="0" cellspacing="0" style="margin-bottom:24px">
                <tr>
                  <td style="padding:8px 0;font-size:14px;color:#64748b;min-width:100px">Email</td>
                  <td style="padding:8px 0;font-size:14px;color:#1e293b;font-weight:500">%s</td>
                </tr>
                <tr>
                  <td style="padding:8px 0;font-size:14px;color:#64748b">Role</td>
                  <td style="padding:8px 0;font-size:14px;color:#1e293b;font-weight:500">%s</td>
                </tr>
              </table>
              <a href="%s" style="display:inline-block;background-color:#2563eb;color:#ffffff;text-decoration:none;font-size:15px;font-weight:600;padding:14px 32px;border-radius:8px">Accept Invitation</a>
              <p style="margin:24px 0 0;font-size:13px;color:#94a3b8;line-height:1.4">
                This invitation expires in 72 hours. If you weren't expecting this, you can safely ignore this email.
              </p>
            </td>
          </tr>
          <!-- Footer -->
          <tr>
            <td style="padding:24px 16px 0;text-align:center;font-size:12px;color:#94a3b8">
              <p style="margin:0 0 4px">Observability Platform &middot; Real-time infrastructure monitoring</p>
              <p style="margin:0">If the button above doesn't work, copy this link into your browser:<br><span style="color:#64748b">%s</span></p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, companyName, inviterName, companyName, roleDisplay, inviteeEmail, roleDisplay, inviteURL, inviteURL)
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
		mv := fmt.Sprintf("%.2f", *metricValue)
		if unit != "" {
			mv += unit
		}
		detailRows += fmt.Sprintf(`                <tr>
                  <td style="padding:10px 0;border-top:1px solid #e2e8f0;font-size:14px;color:#64748b">Current Value</td>
                  <td style="padding:10px 0;border-top:1px solid #e2e8f0;font-size:14px;color:#ef4444;font-weight:600;text-align:right">%s</td>
                </tr>`, mv)
	}
	if thresholdValue != nil {
		tv := fmt.Sprintf("%.2f", *thresholdValue)
		if unit != "" {
			tv += unit
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
