package job

import "testing"

func expect(t *testing.T, want, got interface{}) {
	if want != got {
		t.Errorf("want %q, got: %q", want, got)
	}
}

func TestKIHappyPath(t *testing.T) {
	const (
		user     = "user"
		question = "Password: "
		pw       = "123"
	)
	c := keyboardInteractiveChallenge(user, map[string]string{
		question: pw,
	})
	answers, _ := c(user, "", []string{question}, nil)
	expect(t, 1, len(answers))
	expect(t, pw, answers[0])
}

func TestKINoQuestions(t *testing.T) {
	const (
		user     = "user"
		question = "Password: "
		pw       = "123"
	)
	c := keyboardInteractiveChallenge(user, map[string]string{
		question: pw,
	})
	answers, _ := c(user, "", []string{}, nil)
	expect(t, 0, len(answers))
}

func TestKIUnknownUser(t *testing.T) {
	c := keyboardInteractiveChallenge("user", nil)
	_, err := c("wrong user", "", nil, nil)
	expect(t, errUnknownUser, err)
}
