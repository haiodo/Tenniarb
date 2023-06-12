//
//  TenniarbUITests.swift
//  TenniarbUITests
//
//  Created by Andrey Sobolev on 23.09.2019.
//  Copyright © 2019 Andrey Sobolev. All rights reserved.
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//
//  See the License for the specific language governing permissions and
//  limitations under the License.

import XCTest

class TenniarbUITests: XCTestCase {
    
    override func setUp() {
        // Put setup code here. This method is called before the invocation of each test method in the class.
        
        // In UI tests it is usually best to stop immediately when a failure occurs.
        continueAfterFailure = false
        
        // In UI tests it’s important to set the initial state - such as interface orientation - required for your tests before they run. The setUp method is a good place to do this.
    }
    
    override func tearDown() {
        // Put teardown code here. This method is called after the invocation of each test method in the class.
    }
    
    func testExample() {
        // UI tests must launch the application that they test.
        let app = XCUIApplication()
        app.launchArguments = [
            "-NSTreatUnknownArgumentsAsOpen", "NO",
            "-ApplePersistenceIgnoreState", "YES"
        ]
        app.launch()
        
        let untitledWindow2 = app.windows["Untitled"]
        let splitGroup = untitledWindow2.children(matching: .splitGroup).element
        splitGroup.children(matching: .group).element.click()
        
        let untitledWindow = untitledWindow2
        untitledWindow/*@START_MENU_TOKEN@*/.outlines["WorldTree"].cells/*[[".splitGroups",".scrollViews.outlines[\"WorldTree\"]",".outlineRows.cells",".cells",".outlines[\"WorldTree\"]"],[[[-1,4,2],[-1,1,2],[-1,0,1]],[[-1,4,2],[-1,1,2]],[[-1,3],[-1,2]]],[0,0]]@END_MENU_TOKEN@*/.children(matching: .textField).element.click()
        splitGroup.children(matching: .splitGroup).element.scrollViews.children(matching: .textView).element.click()
        
        let icoComponentCell = untitledWindow/*@START_MENU_TOKEN@*/.outlines["WorldTree"].cells.containing(.image, identifier:"ico component").element/*[[".splitGroups",".scrollViews.outlines[\"WorldTree\"]",".outlineRows.cells.containing(.image, identifier:\"ico component\").element",".cells.containing(.image, identifier:\"ico component\").element",".outlines[\"WorldTree\"]"],[[[-1,4,2],[-1,1,2],[-1,0,1]],[[-1,4,2],[-1,1,2]],[[-1,3],[-1,2]]],[0,0]]@END_MENU_TOKEN@*/
        icoComponentCell.typeText("\r")
        icoComponentCell.typeText("test 1")
        untitledWindow.children(matching: .splitGroup).element.children(matching: .splitGroup).element/*@START_MENU_TOKEN@*/.buttons["add"]/*[[".groups.buttons[\"add\"]",".buttons[\"add\"]"],[[[-1,1],[-1,0]]],[0]]@END_MENU_TOKEN@*/.click()
        
        let icoGroupCell = untitledWindow/*@START_MENU_TOKEN@*/.outlines["WorldTree"].cells.containing(.image, identifier:"ico group").element/*[[".splitGroups",".scrollViews.outlines[\"WorldTree\"]",".outlineRows.cells.containing(.image, identifier:\"ico group\").element",".cells.containing(.image, identifier:\"ico group\").element",".outlines[\"WorldTree\"]"],[[[-1,4,2],[-1,1,2],[-1,0,1]],[[-1,4,2],[-1,1,2]],[[-1,3],[-1,2]]],[0,0]]@END_MENU_TOKEN@*/
        icoGroupCell.typeText("\r")
        icoGroupCell.typeText("color red")
        
    }
    
    func testLaunchPerformance() {
        if #available(macOS 10.15, iOS 13.0, tvOS 13.0, *) {
            // This measures how long it takes to launch your application.
            measure(metrics: [XCTOSSignpostMetric.applicationLaunch]) {
                XCUIApplication().launch()
            }
        }
    }
}
