package mailer

import (
	"fmt"
	"html"
	"strings"
)

type BlueTemplateData struct {
	Title       string
	Preheader   string
	Badge       string
	Greeting    string
	Intro       string
	CodeLabel   string
	Code        string
	CodeHint    string
	DetailLabel string
	DetailValue string
	Warning     string
	Footer      string
}

func BlueTemplate(data BlueTemplateData) string {
	title := safeText(data.Title, "SAKU notification")
	preheader := safeText(data.Preheader, title)
	badge := safeText(data.Badge, "SAKU")
	greeting := safeText(data.Greeting, "Hi,")
	intro := safeText(data.Intro, "")
	codeLabel := safeText(data.CodeLabel, "")
	code := safeText(data.Code, "")
	codeHint := safeText(data.CodeHint, "")
	detailLabel := safeText(data.DetailLabel, "")
	detailValue := safeMultilineText(data.DetailValue)
	warning := safeText(data.Warning, "")
	footer := safeText(data.Footer, "SAKU Finance - Automated email, please do not reply.")

	codeBlock := ""
	if code != "" {
		codeBlock = fmt.Sprintf(`
              <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background:#f8fbff;border:1px solid #dbeafe;border-radius:16px;margin:0 0 18px;">
                <tr>
                  <td align="center" style="padding:24px 20px;">
                    <div style="font-size:11px;font-weight:800;letter-spacing:.18em;color:#2563eb;text-transform:uppercase;margin-bottom:13px;">%s</div>
                    <div style="display:inline-block;padding:14px 28px;border-radius:12px;background:#ffffff;border:1px solid #dbeafe;font-family:'Courier New',Courier,monospace;font-size:34px;font-weight:900;letter-spacing:.22em;line-height:1;color:#1e3a8a;">%s</div>
                    <div style="margin-top:14px;font-size:13px;font-weight:700;color:#2563eb;">%s</div>
                  </td>
                </tr>
              </table>`, codeLabel, code, codeHint)
	}

	detailBlock := ""
	if detailValue != "" {
		detailBlock = fmt.Sprintf(`
              <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background:#f8fbff;border:1px solid #dbeafe;border-radius:14px;margin:0 0 16px;">
                <tr>
                  <td style="padding:14px 16px;">
                    <div style="font-size:10px;font-weight:800;color:#2563eb;margin-bottom:5px;letter-spacing:.10em;text-transform:uppercase;">%s</div>
                    <div style="font-size:13px;font-weight:700;color:#1e3a8a;word-break:break-word;">%s</div>
                  </td>
                </tr>
              </table>`, detailLabel, detailValue)
	}

	warningBlock := ""
	if warning != "" {
		warningBlock = fmt.Sprintf(`
              <div style="border-left:3px solid #2563eb;background:#eff6ff;border-radius:10px;padding:13px 14px;margin:0 0 18px;">
                <p style="margin:0;font-size:13px;line-height:1.7;color:#1e40af;">%s</p>
              </div>`, warning)
	}

	securityNote := ""
	if code != "" {
		securityNote = `<p style="margin:0;font-size:12px;line-height:1.7;color:#6b7280;">SAKU will never ask for your OTP through chat, phone, or email.</p>`
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <meta name="color-scheme" content="light">
  <title>%s</title>
</head>
<body style="margin:0;padding:0;background:#f3f6fb;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Inter,Arial,sans-serif;color:#111827;">
  <div style="display:none;max-height:0;overflow:hidden;mso-hide:all;font-size:1px;color:#f3f6fb;line-height:1px;">%s</div>
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background:#f3f6fb;">
    <tr>
      <td align="center" style="padding:28px 16px;">
        <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="max-width:560px;background:#ffffff;border:1px solid #e5e7eb;border-radius:18px;overflow:hidden;">
          <tr><td style="height:8px;background:#2563eb;line-height:8px;font-size:0;">&nbsp;</td></tr>
          <tr>
            <td style="padding:28px 32px 18px;">
              <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">
                <tr>
                  <td style="vertical-align:middle;">
                    <table role="presentation" cellpadding="0" cellspacing="0" border="0">
                      <tr>
                        <td style="width:46px;height:46px;border-radius:12px;background:#2563eb;text-align:center;vertical-align:middle;">
                          <span style="font-size:21px;font-weight:900;line-height:46px;color:#ffffff;">S</span>
                        </td>
                        <td style="padding-left:13px;vertical-align:middle;">
                          <div style="font-size:22px;font-weight:800;color:#1f2937;letter-spacing:.01em;">SAKU</div>
                          <div style="font-size:11px;font-weight:700;letter-spacing:.12em;color:#2563eb;text-transform:uppercase;">Finance Workspace</div>
                        </td>
                      </tr>
                    </table>
                  </td>
                  <td align="right" style="vertical-align:middle;">
                    <span style="display:inline-block;padding:8px 13px;border-radius:999px;background:#eff6ff;border:1px solid #bfdbfe;font-size:11px;font-weight:800;color:#1d4ed8;text-transform:uppercase;letter-spacing:.06em;">%s</span>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
          <tr>
            <td style="padding:0 32px;">
              <h1 style="margin:20px 0 18px;font-size:28px;font-weight:800;line-height:1.25;color:#374151;">%s</h1>
              <div style="height:1px;background:#e5e7eb;margin:0 0 28px;"></div>
            </td>
          </tr>
          <tr>
            <td style="padding:0 32px 30px;">
              <p style="margin:0 0 22px;font-size:16px;line-height:1.75;color:#374151;">%s</p>
              <p style="margin:0 0 26px;font-size:15px;line-height:1.8;color:#4b5563;">%s</p>
              %s
              %s
              %s
              %s
            </td>
          </tr>
          <tr>
            <td style="padding:20px 32px;border-top:1px solid #e5e7eb;background:#ffffff;">
              <p style="margin:0;font-size:13px;line-height:1.65;color:#4b5563;">%s</p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, title, preheader, badge, title, greeting, intro, codeBlock, detailBlock, warningBlock, securityNote, footer)
}

func safeText(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	return html.EscapeString(value)
}

func safeMultilineText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.ReplaceAll(html.EscapeString(value), "\n", "<br>")
}
