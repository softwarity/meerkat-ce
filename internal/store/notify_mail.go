package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// The notifications' mail queue (NOTIF-05): one recipient row carries its
// own mail, so what is still to send survives the node that took the
// publication. A node claims one due row at a time (compare and set on the
// row: two nodes never send the same mail), sends it, and writes the outcome
// back on the row.

// The states of one reader's mail.
const (
	MailPending   = "pending"
	MailSending   = "sending"
	MailAccepted  = "accepted"
	MailFailed    = "failed"
	MailUnknown   = "unknown"
	MailCancelled = "cancelled"
	MailNoAddress = "noaddress"
	// MailDigest is a mail that goes inside a digest another row sends: one
	// mail for the notifications an account was given at once. Counted as
	// pending until the digest leaves.
	MailDigest = "digest"
)

// MailStates are the states a reader's mail is counted in, in the order a
// sender reads them.
var MailStates = []string{MailPending, MailSending, MailAccepted, MailFailed, MailUnknown, MailCancelled, MailNoAddress}

// QueuedMail is one reader's mail, claimed by a node to be sent.
type QueuedMail struct {
	NotificationID string
	UserID         string
	// Attempts is how many were made before this one.
	Attempts int
	// Deadline is the last instant a temporary refusal is tried again.
	Deadline int64
	// MessageID is the same on every attempt.
	MessageID string
	// Digest names the digest this row sends, "" for a mail of its own.
	Digest string
}

// QueueMails asks for the mails of one notification to the accounts named,
// due at once, a temporary refusal tried again until deadline. messageID
// names each one. A row that already carries a mail is left as it is.
func (s *Store) QueueMails(ctx context.Context, id string, userIDs []string, now, deadline int64,
	messageID func(userID string) string) error {
	for _, u := range userIDs {
		if _, err := s.db.ExecContext(ctx, `UPDATE notification_recipients SET mail_state = ?, mail_next_at = ?,
			mail_deadline = ?, mail_id = ? WHERE notification_id = ? AND user_id = ? AND mail_state = ''`,
			MailPending, now, deadline, messageID(u), id, u); err != nil {
			return fmt.Errorf("store: queue the mail of %q to %q: %w", id, u, err)
		}
	}
	return nil
}

// ClaimMail takes the next mail due by now for node, until now+lease, and
// says whether there was one. Another node that read the same row first wins
// it; this one looks for the next.
func (s *Store) ClaimMail(ctx context.Context, node string, now, lease int64) (QueuedMail, bool, error) {
	for range 8 {
		var m QueuedMail
		err := s.db.QueryRowContext(ctx, `SELECT notification_id, user_id, mail_attempts, mail_deadline, mail_id,
			mail_digest FROM notification_recipients WHERE mail_state = ? AND mail_next_at <= ?
			ORDER BY mail_next_at, notification_id, user_id LIMIT 1`, MailPending, now).
			Scan(&m.NotificationID, &m.UserID, &m.Attempts, &m.Deadline, &m.MessageID, &m.Digest)
		if errors.Is(err, sql.ErrNoRows) {
			return m, false, nil
		}
		if err != nil {
			return m, false, fmt.Errorf("store: claim a notification mail: %w", err)
		}
		res, err := s.db.ExecContext(ctx, `UPDATE notification_recipients SET mail_state = ?, mail_claimed_by = ?,
			mail_claimed_until = ? WHERE notification_id = ? AND user_id = ? AND mail_state = ?`,
			MailSending, node, now+lease, m.NotificationID, m.UserID, MailPending)
		if err != nil {
			return m, false, fmt.Errorf("store: claim a notification mail: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 1 {
			return m, true, nil
		}
	}
	return QueuedMail{}, false, nil
}

// SettleMail writes the outcome of one attempt on the row node claimed:
// MailPending again with nextAt for a temporary refusal, or a final state.
// code and reply are the relay's answer, when it gave one.
func (s *Store) SettleMail(ctx context.Context, node string, m QueuedMail, state string, code int, reply string,
	nextAt int64) error {
	if len(reply) > 300 {
		reply = reply[:300]
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE notification_recipients SET mail_state = ?, mail_attempts = ?,
		mail_next_at = ?, mail_code = ?, mail_error = ?, mail_claimed_by = '', mail_claimed_until = 0
		WHERE notification_id = ? AND user_id = ? AND mail_state = ? AND mail_claimed_by = ?`,
		state, m.Attempts+1, nextAt, code, reply, m.NotificationID, m.UserID, MailSending, node); err != nil {
		return fmt.Errorf("store: settle the mail of %q to %q: %w", m.NotificationID, m.UserID, err)
	}
	return nil
}

// RecoverLostMails settles the mails a node claimed and never answered for -
// it stopped while sending - as unknown: the relay may have taken them, and
// sending them again could deliver them twice. It answers the notifications
// they belong to.
func (s *Store) RecoverLostMails(ctx context.Context, now int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT notification_id FROM notification_recipients
		WHERE mail_state = ? AND mail_claimed_until < ?`, MailSending, now)
	if err != nil {
		return nil, fmt.Errorf("store: recover the notification mails: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	_ = rows.Close()
	if len(ids) == 0 {
		return nil, rows.Err()
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE notification_recipients SET mail_state = ?, mail_error = ?,
		mail_claimed_by = '', mail_claimed_until = 0 WHERE mail_state = ? AND mail_claimed_until < ?`,
		MailUnknown, "the node sending it stopped before the relay answered", MailSending, now); err != nil {
		return nil, fmt.Errorf("store: recover the notification mails: %w", err)
	}
	return ids, nil
}

// CancelMails cancels what one notification has still to send. A mail being
// sent finishes.
func (s *Store) CancelMails(ctx context.Context, id string) error {
	// The row that sends a digest is left to it: the digest may hold other
	// notifications still live, and it says itself what it sent.
	if _, err := s.db.ExecContext(ctx, `UPDATE notification_recipients SET mail_state = ?
		WHERE notification_id = ? AND (mail_state = ? OR (mail_state = ? AND mail_digest = ''))`,
		MailCancelled, id, MailDigest, MailPending); err != nil {
		return fmt.Errorf("store: cancel the mails of %q: %w", id, err)
	}
	return nil
}

// MailCounts counts the mails of each of ids by state: what a sender reads
// next to its acknowledgements.
func (s *Store) MailCounts(ctx context.Context, ids []string) (map[string]map[string]int, error) {
	out := map[string]map[string]int{}
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.QueryContext(ctx, `SELECT notification_id,
		CASE WHEN mail_state = 'digest' THEN 'pending' ELSE mail_state END AS state, COUNT(*)
		FROM notification_recipients WHERE mail_state <> '' AND notification_id IN (?`+strings.Repeat(", ?", len(ids)-1)+`)
		GROUP BY notification_id, CASE WHEN mail_state = 'digest' THEN 'pending' ELSE mail_state END`, args...)
	if err != nil {
		return nil, fmt.Errorf("store: count the notification mails: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, state string
		var n int
		if err := rows.Scan(&id, &state, &n); err != nil {
			return nil, err
		}
		if out[id] == nil {
			out[id] = map[string]int{}
		}
		out[id][state] = n
	}
	return out, rows.Err()
}

// MailFailureCodes counts the reply codes of one notification's failed
// mails, 0 for a relay that never answered.
func (s *Store) MailFailureCodes(ctx context.Context, id string) (map[int]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT mail_code, COUNT(*) FROM notification_recipients
		WHERE notification_id = ? AND mail_state = ? GROUP BY mail_code`, id, MailFailed)
	if err != nil {
		return nil, fmt.Errorf("store: the failed mails of %q: %w", id, err)
	}
	defer func() { _ = rows.Close() }()
	out := map[int]int{}
	for rows.Next() {
		var code, n int
		if err := rows.Scan(&code, &n); err != nil {
			return nil, err
		}
		out[code] = n
	}
	return out, rows.Err()
}

// MailDone marks one notification's mailing reported once nothing is left to
// send, and says whether this call did: the node that settles its last mail
// writes its outcome, once, whichever node that is.
func (s *Store) MailDone(ctx context.Context, id string, now int64) (bool, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE notifications SET mail_reported_at = ?
		WHERE id = ? AND mail_reported_at = 0 AND NOT EXISTS (SELECT 1 FROM notification_recipients
		WHERE notification_id = ? AND mail_state IN (?, ?, ?))`, now, id, id, MailPending, MailSending, MailDigest)
	if err != nil {
		return false, fmt.Errorf("store: report the mails of %q: %w", id, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// QueueDigest asks for ONE mail to an account for the notifications ids it
// was given at once: the first row sends it, the others wait inside it.
func (s *Store) QueueDigest(ctx context.Context, userID string, ids []string, digest, messageID string,
	now, deadline int64) error {
	for i, id := range ids {
		state := MailDigest
		if i == 0 {
			state = MailPending
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE notification_recipients SET mail_state = ?, mail_next_at = ?,
			mail_deadline = ?, mail_id = ?, mail_digest = ? WHERE notification_id = ? AND user_id = ? AND mail_state = ''`,
			state, now, deadline, messageID, digest, id, userID); err != nil {
			return fmt.Errorf("store: queue the digest of %q: %w", userID, err)
		}
	}
	return nil
}

// DigestRows are the notifications one digest of an account carries, the
// row that sends it first.
func (s *Store) DigestRows(ctx context.Context, userID, digest string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT notification_id FROM notification_recipients
		WHERE user_id = ? AND mail_digest = ? AND mail_state IN (?, ?, ?)
		ORDER BY CASE WHEN mail_state = ? THEN 1 ELSE 0 END, notification_id`,
		userID, digest, MailPending, MailSending, MailDigest, MailDigest)
	if err != nil {
		return nil, fmt.Errorf("store: the digest %q of %q: %w", digest, userID, err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SettleDigest writes on the rows waiting inside a digest what became of
// it: the outcome of the mail for ids, cancelled for the others.
func (s *Store) SettleDigest(ctx context.Context, userID, digest string, ids []string, state string, code int,
	reply string) error {
	if len(reply) > 300 {
		reply = reply[:300]
	}
	rows, err := s.DigestRows(ctx, userID, digest)
	if err != nil {
		return err
	}
	for _, id := range rows {
		st, c, r := MailCancelled, 0, ""
		for _, sent := range ids {
			if sent == id {
				st, c, r = state, code, reply
			}
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE notification_recipients SET mail_state = ?, mail_code = ?,
			mail_error = ? WHERE notification_id = ? AND user_id = ? AND mail_digest = ? AND mail_state = ?`,
			st, c, r, id, userID, digest, MailDigest); err != nil {
			return fmt.Errorf("store: settle the digest %q of %q: %w", digest, userID, err)
		}
	}
	return nil
}
