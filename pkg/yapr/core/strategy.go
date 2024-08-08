package core

import (
	"noy/router/pkg/yapr/core/errcode"
	"noy/router/pkg/yapr/core/strategy"
)

var strategyBuilders = make(map[string]strategy.StrategyBuilder)

func RegisterStrategyBuilder(name string, builder strategy.StrategyBuilder) {
	strategyBuilders[name] = builder
}

func GetStrategy(selector *Selector) (strategy.Strategy, error) {
	builder, ok := strategyBuilders[selector.Strategy]
	if !ok {
		return nil, errcode.ErrUnknownStrategy
	}
	return builder.Build(selector.Selector)
}
