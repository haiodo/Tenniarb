//
//  SampleElementModels.swift
//  TenniarbTests
//
//  Created by Andrey Sobolev on 15/10/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
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

import Foundation

@testable import Tenniarb

public class TestElementFactory {
    public static func createModel () -> ElementModel {
        let elementModel = ElementModel()
        
        let pl = Element(name: "platform", createSelf: true)
        elementModel.add(pl)
        
        let index = pl.add( makeItem: Element(name: "Index"))
        index.x = -84
        index.y = 102
        
        let st = pl.add( makeItem: Element(name: "StateTracker"))
        st.x = -189
        st.y = -99
        
        let dt = Element(name: "DeviceTracker", createSelf: true)
        let dte = pl.add( makeItem: dt )
        dte.x = 129
        dte.y = 48
        
        let dev = dt.add( makeItem: Element(name: "Device"))
        dev.x = -50
        dev.y = 50
        
        let repo = Element(name: "Repository", createSelf: true)
        let repoe = pl.add( makeItem: repo )
        repoe.x = 56
        repoe.y = -109
        
        let dbe = Element(name: "Database")
        let dbei = repo.add( makeItem: dbe )
        
        pl.add(source: repoe, target: dbei)
        dbei.x = 126
        dbei.y = -216
        
        
        // Add small just platform diagram.
        
        let dm = DiagramItem(kind:.Item, name: "DataModel")
        dm.x = -100
        dm.y = -200
        pl.add( dm )
        
        let str = DiagramItem(kind:.Item, name: "Structure")
        pl.add( source: dm, target: str )
        pl.add( source: str, target: DiagramItem(kind:.Item, name: "Elements"))
        pl.add( source: str, target: DiagramItem(kind:.Item, name: "DiagramItems"))
    
        return elementModel
    }
}
