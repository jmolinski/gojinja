package stack

type Stack[K any] []K

func New[K any]() *Stack[K] {
	s := make(Stack[K], 0)
	return &s
}

func (stack Stack[K]) Peek() *K {
	if len(stack) == 0 {
		return nil
	}
	return &stack[len(stack)-1]
}

func (stack *Stack[K]) Pop() *K {
	if len(*stack) == 0 {
		return nil
	}
	top := (*stack)[len(*stack)-1]
	*stack = (*stack)[:len(*stack)-1]
	return &top
}

func (stack *Stack[K]) Push(el K) {
	*stack = append(*stack, el)
}
