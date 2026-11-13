In `apps/empty.tenn`, inside element `Empty`, add a 480x600 dialog at `pos 0 0`: white, corner radius 12, column layout; a 64 high header padded
0 24 with the 18 pt bold title `Invite people`; a body that fills the remaining height, padded 24,
with two `Libraries.forms.Field` uses `Email` = `maria@acme.dev` and `Role` = `Editor`; and a 72
high footer padded 0 24 with a row at the end, gap 8: a secondary `Cancel` and a primary `Send
invite`. The footer stays at the bottom when the body grows. Change nothing else.
