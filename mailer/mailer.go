package mailer

import (
	"io"

	"github.com/samber/oops"
	mail "github.com/wneessen/go-mail"
)

// via https://go-mail.dev/getting-started/introduction/

type Mailer struct {
	Host     string
	Username string
	Password string
	From string
}

func (m *Mailer) Send(to string, subject string, attachment io.ReadSeeker) (bool, error) {
	oopsBuilder := oops.In("Mailer::Send")
	msg := mail.NewMsg()
	if err := msg.From(m.From); err != nil {
		return false, oopsBuilder.Wrap(err)
	}
	if err := msg.To(to); err != nil {
		return false, oopsBuilder.Wrap(err)
	}
	msg.Subject(subject)
	msg.SetDate()
	if attachment != nil {
		msg.AttachReadSeeker("report.pdf", attachment)
	}
	msg.SetBodyString("text/plain", "This is an email!")
	
	client, err := mail.NewClient(
		m.Host,
		mail.WithUsername(m.Username),
		mail.WithPassword(m.Password),
		mail.WithSSL(),
		mail.WithSMTPAuth(mail.SMTPAuthLogin),
		mail.WithDebugLog(),
	)
	if err != nil {
		return false, oopsBuilder.Wrap(err)
	}
	defer client.Close()

	err = client.DialAndSend(msg)
	if err != nil {
		err = oops.Wrap(err)
	}

	return err != nil, err
}