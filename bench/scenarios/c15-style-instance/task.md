In `apps/empty.tenn`, inside element `Empty`, add a `styles { accent { color #6b3ad3 } }` block to element `Empty`, then a row at
`pos 0 0` with gap 16 holding two `Libraries.ui_kit.Card` uses titled `Plain` and `Accented`. The
second one gets `use-style accent`, so its surface is `#6b3ad3`; the first stays white. Do not change
`lib/ui_kit.tenn`.
