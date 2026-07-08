package mailer

import (
	"log"
)

type Message struct {
	To      string
	Subject string
	Body    string
}

type AsyncMailer struct {
	next Mailer
	jobs chan Message
}

func NewAsync(next Mailer, buffer int) Mailer {
	if buffer <= 0 {
		buffer = 100
	}
	m := &AsyncMailer{next: next, jobs: make(chan Message, buffer)}
	go m.worker()
	return m
}

func (m *AsyncMailer) Send(to, subject, body string) error {
	job := Message{To: to, Subject: subject, Body: body}
	select {
	case m.jobs <- job:
		log.Printf("mailer broker: queued to=%q subject=%q", to, subject)
	default:
		log.Printf("mailer broker: queue full, deferring enqueue to=%q subject=%q", to, subject)
		go func() {
			m.jobs <- job
		}()
	}
	return nil
}

func (m *AsyncMailer) worker() {
	for job := range m.jobs {
		if err := m.next.Send(job.To, job.Subject, job.Body); err != nil {
			log.Printf("mailer broker: send failed to=%q subject=%q err=%v", job.To, job.Subject, err)
		}
	}
}
