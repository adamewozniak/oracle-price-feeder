package provider

import (
	"context"
	"testing"

	"price-feeder/oracle/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestCryptoProvider_GetTickerPrices(t *testing.T) {
	p, err := NewCryptoProvider(
		context.TODO(),
		zerolog.Nop(),
		Endpoint{},
		types.CurrencyPair{Base: "ATOM", Quote: "USDT"},
	)
	require.NoError(t, err)

	t.Run("valid_request_single_ticker", func(t *testing.T) {
		lastPrice := sdk.MustNewDecFromStr("34.69000000")
		volume := sdk.MustNewDecFromStr("2396974.02000000")

		tickerMap := map[string]types.TickerPrice{}
		tickerMap["ATOM_USDT"] = types.TickerPrice{
			Price:  lastPrice,
			Volume: volume,
		}

		p.tickers = tickerMap

		prices, err := p.GetTickerPrices(types.CurrencyPair{Base: "ATOM", Quote: "USDT"})
		require.NoError(t, err)
		require.Len(t, prices, 1)
		require.Equal(t, lastPrice, prices["ATOMUSDT"].Price)
		require.Equal(t, volume, prices["ATOMUSDT"].Volume)
	})

	t.Run("valid_request_multi_ticker", func(t *testing.T) {
		lastPriceAtom := sdk.MustNewDecFromStr("34.69000000")
		lastPriceLuna := sdk.MustNewDecFromStr("41.35000000")
		volume := sdk.MustNewDecFromStr("2396974.02000000")

		tickerMap := map[string]types.TickerPrice{}
		tickerMap["ATOM_USDT"] = types.TickerPrice{
			Price:  lastPriceAtom,
			Volume: volume,
		}

		tickerMap["LUNA_USDT"] = types.TickerPrice{
			Price:  lastPriceLuna,
			Volume: volume,
		}

		p.tickers = tickerMap
		prices, err := p.GetTickerPrices(
			types.CurrencyPair{Base: "ATOM", Quote: "USDT"},
			types.CurrencyPair{Base: "LUNA", Quote: "USDT"},
		)
		require.NoError(t, err)
		require.Len(t, prices, 2)
		require.Equal(t, lastPriceAtom, prices["ATOMUSDT"].Price)
		require.Equal(t, volume, prices["ATOMUSDT"].Volume)
		require.Equal(t, lastPriceLuna, prices["LUNAUSDT"].Price)
		require.Equal(t, volume, prices["LUNAUSDT"].Volume)
	})

	t.Run("invalid_request_invalid_ticker", func(t *testing.T) {
		prices, err := p.GetTickerPrices(types.CurrencyPair{Base: "FOO", Quote: "BAR"})
		require.Error(t, err)
		require.Equal(t, "crypto failed to get ticker price for FOO_BAR", err.Error())
		require.Nil(t, prices)
	})
}

func TestCryptoCurrencyPairToCryptoPair(t *testing.T) {
	cp := types.CurrencyPair{Base: "ATOM", Quote: "USDT"}
	cryptoSymbol := cp.Join("_")
	require.Equal(t, cryptoSymbol, "ATOM_USDT")
}
