//
//  AppDelegate.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 26/05/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Cocoa
import StoreKit

@NSApplicationMain
class AppDelegate: NSObject, NSApplicationDelegate {
    var terminateOnLastWindowClose = true

    func applicationWillTerminate(_ aNotification: Notification) {
        // Insert code here to tear down your application
    }
    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        return terminateOnLastWindowClose
    }
    
    public func setTerminateWindows(_ value: Bool ) {
        self.terminateOnLastWindowClose = value
    }
    
    func applicationDidBecomeActive(_ notification: Notification) {
        Swift.print("Launching application")
    }
 }

