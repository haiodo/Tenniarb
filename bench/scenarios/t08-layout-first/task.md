`layout` reports an overflow in `apps/empty.tenn`: a 480 wide card sits inside a 400 wide column.
Fix it so `layout` reports no problem — either narrow the card or widen the column — and check with
`layout` before rendering. Change nothing else.
