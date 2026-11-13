In `apps/empty.tenn`, inside element `Empty`, add a 400 wide card at `pos 0 0`: white, corner radius 12, padding 24, column gap 12, title
`Payment method` at 18 pt bold. Below it a `Card number` label at 12 pt `#6b7280`, a 40 high field
with a 2 px `#b42318` border and radius 8 holding `4242 4242 4242 4242`, and under the field a 13 pt
`#b42318` text `This card was declined.`, then a primary `Pay` button that fills the width. Do not use
`Libraries.forms.Field` for the card number: its border is the default one. Change nothing else.
