package nes

/*
bit:	7	6	5	4	3	2	1	0
button:	A	B	Select	Start	Up	Down	Left	Right
*/

/*
	只能往 4016 写（写 4017 给 APU 用了），
	读可以往 4016 和 4017 读。写 4016 时，对两个手柄都有效，
	读时则 4016 为 P1，4017 为 P2

	给4016写,同时写两个手柄
	读则4016 为 P1，4017 为 P2

	strobe 是选通

*/

const (
	ButtonA = iota
	ButtonB
	ButtonSelect
	ButtonStart
	ButtonUp
	ButtonDown
	ButtonLeft
	ButtonRight
)

type Controller struct {
	buttons [8]bool
	index   byte
	strobe  byte
}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) SetButtons(buttons [8]bool) {
	c.buttons = buttons
}

func (c *Controller) Read() byte {
	value := byte(0)
	if c.index < 8 && c.buttons[c.index] {
		value = 1
	}
	c.index++
	if c.strobe&1 == 1 {
		c.index = 0
	}
	return value
}

func (c *Controller) Write(value byte) {
	c.strobe = value
	if c.strobe&1 == 1 {
		c.index = 0
	}
}
