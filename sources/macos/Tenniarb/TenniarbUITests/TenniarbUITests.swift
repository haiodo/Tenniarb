//
//  TenniarbUITests.swift
//  TenniarbUITests
//
//  Created by Andrey Sobolev on 24/02/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
//

import XCTest
import Cocoa

@testable import Tenniarb

class TenniarbUITests: XCTestCase {
        
    override func setUp() {
        super.setUp()
        
        // Put setup code here. This method is called before the invocation of each test method in the class.
        
        // In UI tests it is usually best to stop immediately when a failure occurs.
        continueAfterFailure = false
        // UI tests must launch the application that they test. Doing this in setup will make sure it happens for each test method.
        let app = XCUIApplication()
        
        let delegate = NSApplication.shared.delegate as! AppDelegate
        delegate.setTerminateWindows(false)
        
        for w in app.windows.allElementsBoundByIndex {
            w.buttons[XCUIIdentifierCloseWindow].click()
        }

        // In UI tests it’s important to set the initial state - such as interface orientation - required for your tests before they run. The setUp method is a good place to do this.
    }
    
    override func tearDown() {
        // Put teardown code here. This method is called after the invocation of each test method in the class.
        super.tearDown()
    }
    
    func testExample() {
        // Use recording to get started writing UI tests.
        // Use XCTAssert and related functions to verify your tests produce the correct results.
        
        let app = XCUIApplication()
        let menuBarsQuery = app.menuBars
        menuBarsQuery.menuBarItems["File"].click()
        menuBarsQuery/*@START_MENU_TOKEN@*/.menuItems["New"]/*[[".menuBarItems[\"File\"]",".menus.menuItems[\"New\"]",".menuItems[\"New\"]"],[[[-1,2],[-1,1],[-1,0,1]],[[-1,2],[-1,1]]],[0]]@END_MENU_TOKEN@*/.click()
        app.windows["Brain Map:Untitled 1"].buttons[XCUIIdentifierCloseWindow].click()
        
        
    }
    
}
