//
//  Application.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 07.06.2019.
//  Copyright © 2019 Andrey Sobolev. All rights reserved.
//

import Foundation
import Cocoa

public class TenniarbApplication: NSApplication {
    override public func sendEvent(_ event: NSEvent) {
        if event.type == NSEvent.EventType.keyDown {
            if (event.modifierFlags.contains(NSEvent.ModifierFlags.command)) {
                switch event.charactersIgnoringModifiers!.lowercased() {
                case "x":
                    if NSApp.sendAction(#selector(NSText.cut(_:)), to:nil, from:self) { return }
                case "c":
                    if NSApp.sendAction(#selector(NSText.copy(_:)), to:nil, from:self) { return }
                case "v":
                    if NSApp.sendAction(#selector(NSText.paste(_:)), to:nil, from:self) { return }
                case "a":
                    if NSApp.sendAction(#selector(NSText.selectAll(_:)), to:nil, from:self) { return }
                default:
                    break
                }
            }
        }
        return super.sendEvent(event)
    }
}
