package prompt

type Gateway struct {
	Builder   *Builder
	Renderer  *Renderer
	Validator *Validator
}

func NewGateway() *Gateway {
	return &Gateway{
		Builder:   NewBuilder(),
		Renderer:  NewRenderer(),
		Validator: NewValidator(),
	}
}

func (g *Gateway) BuildMessages(req BuildRequest) ([]GwMessage, error) {
	ir := g.Builder.Build(req)

	if err := g.Validator.ValidateIR(ir); err != nil {
		return nil, err
	}

	messages, err := g.Renderer.Render(ir)
	if err != nil {
		return nil, err
	}

	if err := g.Validator.ValidateMessages(messages); err != nil {
		return nil, err
	}

	return messages, nil
}
