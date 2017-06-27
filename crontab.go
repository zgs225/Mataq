package maatq

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var (
	cronMonthMap = map[string]int{
		"jan": 1,
		"feb": 2,
		"mar": 3,
		"apr": 4,
		"may": 5,
		"jun": 6,
		"jul": 7,
		"aug": 8,
		"sep": 9,
		"oct": 10,
		"nov": 11,
		"dec": 12,
	}
	cronWeekMap = map[string]int{
		"sun": 0,
		"mon": 1,
		"tue": 2,
		"wed": 3,
		"thu": 4,
		"fri": 5,
		"sat": 6,
	}
)

type Crontab struct {
	Minutes     []int8 // 0 - 59
	Hours       []int8 // 0 - 23
	DaysOfMonth []int8 // 0 - 31
	Months      []int8 // 0 - 12
	DaysOfWeek  []int8 // 0 - 7 (0或者7是周日, 或者使用名字)
	Text        string // Cron字符串
}

func (cron *Crontab) Next() time.Time {
	return cron.nextFrom(time.Now())
}

func (cron *Crontab) nextFrom(from time.Time) time.Time {
	var (
		next time.Time
		done bool
	)
	next = from.Add(time.Minute)
	next = time.Date(next.Year(), next.Month(), next.Day(), next.Hour(), next.Minute(), 0, 0, next.Location())
	for !done {
		if !inInt8Slice(int8(next.Month()), cron.Months) {
			m := next.Month() + 1
			y := next.Year()
			if m > 12 {
				m = 1
				y = y + 1
			}
			next = time.Date(y, m, 1, 0, 0, 0, 0, next.Location())
			continue
		}

		if !inInt8Slice(int8(next.Day()), cron.DaysOfMonth) && !inInt8Slice(int8(next.Weekday()), cron.DaysOfWeek) {
			next = next.Add(time.Hour * 24)
			next = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, next.Location())
			continue
		}

		if !inInt8Slice(int8(next.Hour()), cron.Hours) {
			next = next.Add(time.Hour)
			next = time.Date(next.Year(), next.Month(), next.Day(), next.Hour(), 0, 0, 0, next.Location())
			continue
		}

		if !inInt8Slice(int8(next.Minute()), cron.Minutes) {
			next = next.Add(time.Minute)
			next = time.Date(next.Year(), next.Month(), next.Day(), next.Hour(), next.Minute(), 0, 0, next.Location())
			continue
		}
		done = true
	}
	return next
}

type cronTokenType int32

const (
	cronTokenTypes_Number cronTokenType = iota
	cronTokenTypes_Word
	cronTokenTypes_Asterisk
	cronTokenTypes_Hyphen
	cronTokenTypes_Slash
	cronTokenTypes_Comma
	cronTokenTypes_EOF
)

func (t cronTokenType) TokenName() string {
	switch t {
	case cronTokenTypes_Number:
		return "<NUM>"
	case cronTokenTypes_Word:
		return "<WORD>"
	case cronTokenTypes_Asterisk:
		return "<ASTERISK>"
	case cronTokenTypes_Hyphen:
		return "<HYPHEN>"
	case cronTokenTypes_Slash:
		return "<SLASH>"
	case cronTokenTypes_Comma:
		return "<COMMA>"
	case cronTokenTypes_EOF:
		return "<EOF>"
	default:
		return "<UNKNOWN>"
	}
}

type cronToken struct {
	t cronTokenType
	v []byte
}

func (t *cronToken) IntVal() (int, error) {
	return strconv.Atoi(string(t.v))
}

func (t *cronToken) StrVal() string {
	return string(t.v)
}

type cronLexer struct {
	b   byte
	r   io.ByteScanner
	err error
}

func (lexer *cronLexer) readByte() {
	lexer.b, lexer.err = lexer.r.ReadByte()
}

func (lexer *cronLexer) NextToken() (*cronToken, error) {
	lexer.readByte()
	if lexer.err != nil {
		goto return_errors
	}
	if unicode.IsSpace(rune(lexer.b)) {
		for {
			lexer.readByte()
			if lexer.err != nil {
				goto return_errors
			}
			if !unicode.IsSpace(rune(lexer.b)) {
				break
			}
		}
	}

	if unicode.IsDigit(rune(lexer.b)) {
		b := new(bytes.Buffer)
		lexer.err = b.WriteByte(lexer.b)
		if lexer.err != nil {
			goto return_errors
		}
		for {
			lexer.readByte()
			if !unicode.IsDigit(rune(lexer.b)) {
				lexer.r.UnreadByte()
				return &cronToken{
					t: cronTokenTypes_Number,
					v: b.Bytes(),
				}, nil
			} else {
				lexer.err = b.WriteByte(lexer.b)
				if lexer.err != nil {
					goto return_errors
				}
			}
		}
	}

	if lexer.b == '*' {
		return &cronToken{
			t: cronTokenTypes_Asterisk,
			v: []byte{lexer.b},
		}, nil
	}

	if lexer.b == '-' {
		return &cronToken{
			t: cronTokenTypes_Hyphen,
			v: []byte{lexer.b},
		}, nil
	}

	if lexer.b == '/' {
		return &cronToken{
			t: cronTokenTypes_Slash,
			v: []byte{lexer.b},
		}, nil
	}

	if lexer.b == ',' {
		return &cronToken{
			t: cronTokenTypes_Comma,
			v: []byte{lexer.b},
		}, nil
	}

	if unicode.IsLetter(rune(lexer.b)) {
		b := new(bytes.Buffer)
		lexer.err = b.WriteByte(lexer.b)
		if lexer.err != nil {
			goto return_errors
		}
		for {
			lexer.readByte()
			if !unicode.IsLetter(rune(lexer.b)) {
				lexer.r.UnreadByte()
				return &cronToken{
					t: cronTokenTypes_Word,
					v: b.Bytes(),
				}, nil
			} else {
				lexer.err = b.WriteByte(lexer.b)
				if lexer.err != nil {
					goto return_errors
				}
			}
		}
	}

	return nil, fmt.Errorf("语法错误: 不支持字符%c", lexer.b)

return_errors:
	if lexer.err == io.EOF {
		return &cronToken{
			t: cronTokenTypes_EOF,
			v: []byte("<EOF>"),
		}, nil
	}
	return nil, lexer.err
}

func newCronLexer(s string) *cronLexer {
	return &cronLexer{
		r: bytes.NewBufferString(s),
	}
}

type cronParser struct {
	lexer     *cronLexer
	lookahead []*cronToken
	cap       int32
	head      int32
}

func (p *cronParser) L(i int32) *cronToken {
	idx := (i + p.head) % p.cap
	return p.lookahead[idx]
}

func (p *cronParser) Consume() (*cronToken, error) {
	t1 := p.L(0)
	p.head = (p.head + 1) % p.cap
	t2, err := p.lexer.NextToken()
	if err != nil {
		return nil, err
	}
	p.lookahead[(p.head+p.cap-1)%p.cap] = t2
	return t1, nil
}

func (p *cronParser) Parse(cron *Crontab) error {
	if err := p.parseMinutes(cron); err != nil {
		return err
	}
	if err := p.parseHours(cron); err != nil {
		return err
	}
	if err := p.parseDaysOfMonth(cron); err != nil {
		return err
	}
	if err := p.parseMonths(cron); err != nil {
		return err
	}
	if err := p.parseDaysOfWeek(cron); err != nil {
		return err
	}
	sort.Sort(Int8Slice(cron.Minutes))
	sort.Sort(Int8Slice(cron.Hours))
	sort.Sort(Int8Slice(cron.DaysOfMonth))
	sort.Sort(Int8Slice(cron.Months))
	sort.Sort(Int8Slice(cron.DaysOfWeek))
	return nil
}

func (p *cronParser) Match(t cronTokenType) error {
	token := p.L(0)
	if token.t != t {
		return fmt.Errorf("语法错误: 应该是%s, 实际: %s, 值: %s", t.TokenName(),
			token.t.TokenName(), string(token.v))
	}
	return nil
}

// */2 形式的结构
// 返回 step int, error
func (p *cronParser) stepedAsterisk() (int, error) {
	if err := p.Match(cronTokenTypes_Asterisk); err != nil {
		return 0, err
	}
	if _, err := p.Consume(); err != nil {
		return 0, err
	}
	if err := p.Match(cronTokenTypes_Slash); err != nil {
		return 0, err
	}
	if _, err := p.Consume(); err != nil {
		return 0, err
	}
	if err := p.Match(cronTokenTypes_Number); err != nil {
		return 0, err
	}
	num, err := p.Consume()
	if err != nil {
		return 0, err
	}
	step, err := num.IntVal()
	if err != nil {
		return 0, err
	}
	return step, nil
}

// *
func (p *cronParser) asterisk() error {
	if err := p.Match(cronTokenTypes_Asterisk); err != nil {
		return err
	}
	if _, err := p.Consume(); err != nil {
		return err
	}
	return nil
}

// 10-30/2
// 返回 begin, end, step int value
func (p *cronParser) stepedRange() (int, int, int, error) {
	// Number
	if err := p.Match(cronTokenTypes_Number); err != nil {
		return 0, 0, 0, err
	}
	begin, err := p.Consume()
	if err != nil {
		return 0, 0, 0, err
	}
	v1, err := begin.IntVal()
	if err != nil {
		return 0, 0, 0, err
	}
	// Hyphen
	if err := p.Match(cronTokenTypes_Hyphen); err != nil {
		return 0, 0, 0, err
	}
	_, err = p.Consume()
	if err != nil {
		return 0, 0, 0, err
	}
	// Number
	if err := p.Match(cronTokenTypes_Number); err != nil {
		return 0, 0, 0, err
	}
	end, err := p.Consume()
	if err != nil {
		return 0, 0, 0, err
	}
	v2, err := end.IntVal()
	if err != nil {
		return 0, 0, 0, err
	}
	// Slash
	if err := p.Match(cronTokenTypes_Slash); err != nil {
		return 0, 0, 0, err
	}
	_, err = p.Consume()
	if err != nil {
		return 0, 0, 0, err
	}
	// Number
	if err := p.Match(cronTokenTypes_Number); err != nil {
		return 0, 0, 0, err
	}
	step, err := p.Consume()
	if err != nil {
		return 0, 0, 0, err
	}
	v3, err := step.IntVal()
	if err != nil {
		return 0, 0, 0, err
	}
	return v1, v2, v3, nil
}

// 10-30
// 返回 begin, end cron token
func (p *cronParser) cronRange() (int, int, error) {
	if err := p.Match(cronTokenTypes_Number); err != nil {
		return 0, 0, err
	}
	begin, err := p.Consume()
	if err != nil {
		return 0, 0, err
	}
	v1, err := begin.IntVal()
	if err != nil {
		return 0, 0, err
	}
	if err := p.Match(cronTokenTypes_Hyphen); err != nil {
		return 0, 0, err
	}
	if _, err := p.Consume(); err != nil {
		return 0, 0, err
	}
	if err := p.Match(cronTokenTypes_Number); err != nil {
		return 0, 0, err
	}
	end, err := p.Consume()
	if err != nil {
		return 0, 0, err
	}
	v2, err := end.IntVal()
	if err != nil {
		return 0, 0, err
	}
	return v1, v2, nil
}

// 3
func (p *cronParser) number() (int, error) {
	if err := p.Match(cronTokenTypes_Number); err != nil {
		return 0, err
	}
	token, err := p.Consume()
	if err != nil {
		return 0, err
	}
	return token.IntVal()
}

// Jan
func (p *cronParser) word() (string, error) {
	if err := p.Match(cronTokenTypes_Word); err != nil {
		return "", err
	}
	token, err := p.Consume()
	if err != nil {
		return "", err
	}
	return token.StrVal(), nil
}

func (p *cronParser) parseMinutes(cron *Crontab) error {
	head := p.L(0)
	// 当分钟是 *
	if head.t == cronTokenTypes_Asterisk {
		if p.L(1).t == cronTokenTypes_Slash { // 当分钟是 */2 类似的模式
			step, err := p.stepedAsterisk()
			if err != nil {
				return err
			}
			cron.Minutes = makeRangeOfInt8(int8(0), int8(59), step)
			return nil
		} else { // 当分钟是 *
			if err := p.asterisk(); err != nil {
				return err
			}
			cron.Minutes = makeRangeOfInt8(int8(0), int8(59), 1)
			return nil
		}
	} else if head.t == cronTokenTypes_Number {
		if p.L(1).t == cronTokenTypes_Hyphen { // 当分钟是 0-59
			if p.L(3).t == cronTokenTypes_Slash { // 是 0-59/3
				v1, v2, v3, err := p.stepedRange()
				if err != nil {
					return err
				}
				if v1 < 0 || v1 > 59 {
					return fmt.Errorf("语法错误: 分钟取值范围是0-59, 实际: %d", v1)
				}
				if v2 < 0 || v2 > 59 {
					return fmt.Errorf("语法错误: 分钟取值范围是0-59, 实际: %d", v2)
				}
				if v1 > v2 {
					return fmt.Errorf("语法错误: 分钟取值范围错误, 实际: %d-%d", v1, v2)
				}
				cron.Minutes = makeRangeOfInt8(int8(v1), int8(v2), v3)
			} else {
				v1, v2, err := p.cronRange()
				if err != nil {
					return err
				}
				if v1 < 0 || v1 > 59 {
					return fmt.Errorf("语法错误: 分钟取值范围是0-59, 实际: %d", v1)
				}
				if v2 < 0 || v2 > 59 {
					return fmt.Errorf("语法错误: 分钟取值范围是0-59, 实际: %d", v2)
				}
				if v1 > v2 {
					return fmt.Errorf("语法错误: 分钟取值范围错误, 实际: %d-%d", v1, v2)
				}
				cron.Minutes = makeRangeOfInt8(int8(v1), int8(v2), 1)
			}
		} else if p.L(1).t == cronTokenTypes_Comma { // 当分钟是 0,13,20
			var minutes []int8
			if err := p.list(&minutes); err != nil {
				return err
			}
			cron.Minutes = minutes
			return nil
		} else { // 单纯的数字
			v, err := p.number()
			if err != nil {
				return err
			}
			if v < 0 || v > 59 {
				return fmt.Errorf("语法错误: 分钟取值范围是0-59, 实际: %d", v)
			}
			cron.Minutes = []int8{int8(v)}
		}
		return nil
	} else {
		return fmt.Errorf("语法错误: 应该是%s或者%s，实际: %s，值: %s",
			cronTokenTypes_Asterisk.TokenName(), cronTokenTypes_Number.TokenName(),
			head.t.TokenName(), string(head.v))
	}
}

func (p *cronParser) parseHours(cron *Crontab) error {
	head := p.L(0)
	// *
	if head.t == cronTokenTypes_Asterisk {
		if p.L(1).t == cronTokenTypes_Slash { // */2 类似的模式
			step, err := p.stepedAsterisk()
			if err != nil {
				return err
			}
			cron.Hours = makeRangeOfInt8(int8(0), int8(23), step)
			return nil
		} else { // *
			if err := p.asterisk(); err != nil {
				return err
			}
			cron.Hours = makeRangeOfInt8(int8(0), int8(23), 1)
			return nil
		}
	} else if head.t == cronTokenTypes_Number {
		if p.L(1).t == cronTokenTypes_Hyphen { // 0-59
			if p.L(3).t == cronTokenTypes_Slash { // 0-59/3
				v1, v2, v3, err := p.stepedRange()
				if err != nil {
					return err
				}
				if v1 < 0 || v1 > 23 {
					return fmt.Errorf("语法错误: 小时取值范围是0-23, 实际: %d", v1)
				}
				if v2 < 0 || v2 > 23 {
					return fmt.Errorf("语法错误: 小时取值范围是0-23, 实际: %d", v2)
				}
				if v1 > v2 {
					return fmt.Errorf("语法错误: 小时取值范围错误, 实际: %d-%d", v1, v2)
				}
				cron.Hours = makeRangeOfInt8(int8(v1), int8(v2), v3)
			} else {
				v1, v2, err := p.cronRange()
				if err != nil {
					return err
				}
				if v1 < 0 || v1 > 23 {
					return fmt.Errorf("语法错误: 小时取值范围是0-23, 实际: %d", v1)
				}
				if v2 < 0 || v2 > 23 {
					return fmt.Errorf("语法错误: 小时取值范围是0-23, 实际: %d", v2)
				}
				if v1 > v2 {
					return fmt.Errorf("语法错误: 小时取值范围错误, 实际: %d-%d", v1, v2)
				}
				cron.Hours = makeRangeOfInt8(int8(v1), int8(v2), 1)
			}
		} else if p.L(1).t == cronTokenTypes_Comma { // 0,13,20
			var hours []int8
			if err := p.list(&hours); err != nil {
				return err
			}
			cron.Hours = hours
			return nil
		} else { // 单纯的数字
			v, err := p.number()
			if err != nil {
				return err
			}
			if v < 0 || v > 23 {
				return fmt.Errorf("语法错误: 小时取值范围是0-23, 实际: %d", v)
			}
			cron.Hours = []int8{int8(v)}
		}
		return nil
	} else {
		return fmt.Errorf("语法错误: 应该是%s或者%s，实际: %s，值: %s",
			cronTokenTypes_Asterisk.TokenName(), cronTokenTypes_Number.TokenName(),
			head.t.TokenName(), string(head.v))
	}
}

func (p *cronParser) parseDaysOfWeek(cron *Crontab) error {
	head := p.L(0)
	// *
	if head.t == cronTokenTypes_Asterisk {
		if p.L(1).t == cronTokenTypes_Slash { // */2 类似的模式
			step, err := p.stepedAsterisk()
			if err != nil {
				return err
			}
			cron.DaysOfWeek = makeRangeOfInt8(int8(0), int8(7), step)
			return nil
		} else { // *
			if err := p.asterisk(); err != nil {
				return err
			}
			cron.DaysOfWeek = []int8{}
			return nil
		}
	} else if head.t == cronTokenTypes_Number {
		if p.L(1).t == cronTokenTypes_Hyphen { // 0-59
			if p.L(3).t == cronTokenTypes_Slash { // 0-59/3
				v1, v2, v3, err := p.stepedRange()
				if err != nil {
					return err
				}
				if v1 < 0 || v1 > 7 {
					return fmt.Errorf("语法错误: 周取值范围是0-7, 实际: %d", v1)
				}
				if v2 < 0 || v2 > 7 {
					return fmt.Errorf("语法错误: 周取值范围是0-7, 实际: %d", v2)
				}
				if v1 > v2 {
					return fmt.Errorf("语法错误: 周取值范围错误, 实际: %d-%d", v1, v2)
				}
				cron.DaysOfWeek = makeRangeOfInt8(int8(v1), int8(v2), v3)
			} else {
				v1, v2, err := p.cronRange()
				if err != nil {
					return err
				}
				if v1 < 0 || v1 > 7 {
					return fmt.Errorf("语法错误: 周取值范围是0-7, 实际: %d", v1)
				}
				if v2 < 0 || v2 > 7 {
					return fmt.Errorf("语法错误: 周取值范围是0-7, 实际: %d", v2)
				}
				if v1 > v2 {
					return fmt.Errorf("语法错误: 周取值范围错误, 实际: %d-%d", v1, v2)
				}
				cron.DaysOfWeek = makeRangeOfInt8(int8(v1), int8(v2), 1)
			}
		} else if p.L(1).t == cronTokenTypes_Comma { // 0,13,20
			var daysOfWeek []int8
			if err := p.list(&daysOfWeek); err != nil {
				return err
			}
			cron.DaysOfWeek = daysOfWeek
			return nil
		} else { // 单纯的数字
			v, err := p.number()
			if err != nil {
				return err
			}
			if v < 0 || v > 7 {
				return fmt.Errorf("语法错误: 周取值范围是0-7, 实际: %d", v)
			}
			cron.DaysOfWeek = []int8{int8(v)}
		}
		return nil
	} else if head.t == cronTokenTypes_Word {
		v, err := p.word()
		if err != nil {
			return err
		}
		n, ok := cronWeekMap[strings.ToLower(v)]
		if !ok {
			return fmt.Errorf("语法错误: 周名称错误，实际: %s", v)
		}
		cron.DaysOfWeek = []int8{int8(n)}
		return nil
	} else {
		return fmt.Errorf("语法错误: 应该是%s或者%s，实际: %s，值: %s",
			cronTokenTypes_Asterisk.TokenName(), cronTokenTypes_Number.TokenName(),
			head.t.TokenName(), string(head.v))
	}
}

func (p *cronParser) parseMonths(cron *Crontab) error {
	head := p.L(0)
	// *
	if head.t == cronTokenTypes_Asterisk {
		if p.L(1).t == cronTokenTypes_Slash { // */2 类似的模式
			step, err := p.stepedAsterisk()
			if err != nil {
				return err
			}
			cron.Months = makeRangeOfInt8(int8(0), int8(12), step)
			return nil
		} else { // *
			if err := p.asterisk(); err != nil {
				return err
			}
			cron.Months = makeRangeOfInt8(int8(0), int8(12), 1)
			return nil
		}
	} else if head.t == cronTokenTypes_Number {
		if p.L(1).t == cronTokenTypes_Hyphen { // 0-59
			if p.L(3).t == cronTokenTypes_Slash { // 0-59/3
				v1, v2, v3, err := p.stepedRange()
				if err != nil {
					return err
				}
				if v1 < 0 || v1 > 12 {
					return fmt.Errorf("语法错误: 月取值范围是0-12, 实际: %d", v1)
				}
				if v2 < 0 || v2 > 12 {
					return fmt.Errorf("语法错误: 月取值范围是0-12, 实际: %d", v2)
				}
				if v1 > v2 {
					return fmt.Errorf("语法错误: 月取值范围错误, 实际: %d-%d", v1, v2)
				}
				cron.Months = makeRangeOfInt8(int8(v1), int8(v2), v3)
			} else {
				v1, v2, err := p.cronRange()
				if err != nil {
					return err
				}
				if v1 < 0 || v1 > 12 {
					return fmt.Errorf("语法错误: 月取值范围是0-12, 实际: %d", v1)
				}
				if v2 < 0 || v2 > 12 {
					return fmt.Errorf("语法错误: 月取值范围是0-12, 实际: %d", v2)
				}
				if v1 > v2 {
					return fmt.Errorf("语法错误: 月取值范围错误, 实际: %d-%d", v1, v2)
				}
				cron.Months = makeRangeOfInt8(int8(v1), int8(v2), 1)
			}
		} else if p.L(1).t == cronTokenTypes_Comma { // 0,13,20
			var months []int8
			if err := p.list(&months); err != nil {
				return err
			}
			cron.Months = months
			return nil
		} else { // 单纯的数字
			v, err := p.number()
			if err != nil {
				return err
			}
			if v < 0 || v > 12 {
				return fmt.Errorf("语法错误: 月取值范围是0-12, 实际: %d", v)
			}
			cron.Months = []int8{int8(v)}
		}
		return nil
	} else if head.t == cronTokenTypes_Word {
		v, err := p.word()
		if err != nil {
			return err
		}
		n, ok := cronMonthMap[strings.ToLower(v)]
		if !ok {
			return fmt.Errorf("语法错误: 月名称错误，实际: %s", v)
		}
		cron.Months = []int8{int8(n)}
		return nil
	} else {
		return fmt.Errorf("语法错误: 应该是%s或者%s，实际: %s，值: %s",
			cronTokenTypes_Asterisk.TokenName(), cronTokenTypes_Number.TokenName(),
			head.t.TokenName(), string(head.v))
	}
}

func (p *cronParser) parseDaysOfMonth(cron *Crontab) error {
	head := p.L(0)
	// *
	if head.t == cronTokenTypes_Asterisk {
		if p.L(1).t == cronTokenTypes_Slash { // */2 类似的模式
			step, err := p.stepedAsterisk()
			if err != nil {
				return err
			}
			cron.DaysOfMonth = makeRangeOfInt8(int8(0), int8(31), step)
			return nil
		} else { // *
			if err := p.asterisk(); err != nil {
				return err
			}
			cron.DaysOfMonth = makeRangeOfInt8(int8(0), int8(31), 1)
			return nil
		}
	} else if head.t == cronTokenTypes_Number {
		if p.L(1).t == cronTokenTypes_Hyphen { // 0-59
			if p.L(3).t == cronTokenTypes_Slash { // 0-59/3
				v1, v2, v3, err := p.stepedRange()
				if err != nil {
					return err
				}
				if v1 < 0 || v1 > 31 {
					return fmt.Errorf("语法错误: 天取值范围是0-31, 实际: %d", v1)
				}
				if v2 < 0 || v2 > 31 {
					return fmt.Errorf("语法错误: 天取值范围是0-31, 实际: %d", v2)
				}
				if v1 > v2 {
					return fmt.Errorf("语法错误: 天取值范围错误, 实际: %d-%d", v1, v2)
				}
				cron.DaysOfMonth = makeRangeOfInt8(int8(v1), int8(v2), v3)
			} else {
				v1, v2, err := p.cronRange()
				if err != nil {
					return err
				}
				if v1 < 0 || v1 > 31 {
					return fmt.Errorf("语法错误: 天取值范围是0-31, 实际: %d", v1)
				}
				if v2 < 0 || v2 > 31 {
					return fmt.Errorf("语法错误: 天取值范围是0-31, 实际: %d", v2)
				}
				if v1 > v2 {
					return fmt.Errorf("语法错误: 天取值范围错误, 实际: %d-%d", v1, v2)
				}
				cron.DaysOfMonth = makeRangeOfInt8(int8(v1), int8(v2), 1)
			}
		} else if p.L(1).t == cronTokenTypes_Comma { // 0,13,20
			var days []int8
			if err := p.list(&days); err != nil {
				return err
			}
			cron.DaysOfMonth = days
			return nil
		} else { // 单纯的数字
			v, err := p.number()
			if err != nil {
				return err
			}
			if v < 0 || v > 31 {
				return fmt.Errorf("语法错误: 天取值范围是0-31, 实际: %d", v)
			}
			cron.DaysOfMonth = []int8{int8(v)}
		}
		return nil
	} else {
		return fmt.Errorf("语法错误: 应该是%s或者%s，实际: %s，值: %s",
			cronTokenTypes_Asterisk.TokenName(), cronTokenTypes_Number.TokenName(),
			head.t.TokenName(), string(head.v))
	}
}

func (p *cronParser) list(container *[]int8) error {
	if err := p.Match(cronTokenTypes_Number); err != nil {
		return err
	}
	t1, err := p.Consume()
	if err != nil {
		return err
	}
	v1, err := t1.IntVal()
	if err != nil {
		return err
	}
	if v1 < 0 || v1 > 59 {
		return fmt.Errorf("语法错误: 分钟取值范围是0-59, 实际: %d", v1)
	}
	if err := p.Match(cronTokenTypes_Comma); err != nil {
		return err
	}
	p.Consume()
	*container = append(*container, int8(v1))

	if p.L(1).t == cronTokenTypes_Comma { // Remain list
		return p.list(container)
	} else {
		if err := p.Match(cronTokenTypes_Number); err != nil {
			return err
		}
		t2, err := p.Consume()
		if err != nil {
			return err
		}
		v2, err := t2.IntVal()
		if err != nil {
			return err
		}
		if v2 < 0 || v2 > 59 {
			return fmt.Errorf("语法错误: 分钟取值范围是0-59, 实际: %d", v2)
		}
		*container = append(*container, int8(v2))
		return nil
	}
}

func newCronParser(lexer *cronLexer, cap int32) (*cronParser, error) {
	p := &cronParser{
		lexer:     lexer,
		lookahead: make([]*cronToken, cap, cap),
		cap:       cap,
		head:      0,
	}
	for i := 0; int32(i) < cap; i++ {
		t, err := lexer.NextToken()
		if err != nil {
			return nil, err
		}
		p.lookahead[i] = t
	}
	return p, nil
}

// 将Crontab的字符串解析成*Crontab实例
func NewCrontab(cron string) (*Crontab, error) {
	var v = Crontab{
		Text: cron,
	}

	lexer := newCronLexer(cron)
	parser, err := newCronParser(lexer, 5)
	if err != nil {
		return nil, err
	}

	if err := parser.Parse(&v); err != nil {
		return nil, err
	}

	return &v, nil
}
