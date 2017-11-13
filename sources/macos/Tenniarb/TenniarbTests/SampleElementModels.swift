//
//  SampleElementModels.swift
//  TenniarbTests
//
//  Created by Andrey Sobolev on 15/10/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Foundation

@testable import Tenniarb

public class TestElementFactory {
    public static func createModel () -> ElementModel {
        let elementModel = ElementModel()
        
        let pl = Element(name: "platform", createSelf: true)
        _ = elementModel.add(pl)
        
        pl.selfItem?.x = 0
        pl.selfItem?.y = 0
        
        let index = pl.add( Element(name: "Index"))
        index.x = -84
        index.y = 102
        
        let st = pl.add( Element(name: "StateTracker"))
        st.x = -189
        st.y = -99
        
        let dt = Element(name: "DeviceTracker", createSelf: true)
        let dte = pl.add( dt )
        dte.x = 129
        dte.y = 48
        
        let dev = dt.add( Element(name: "Device"))
        dev.x = -50
        dev.y = 50
        
        let repo = Element(name: "Repository", createSelf: true)
        let repoe = pl.add( repo )
        repoe.x = 56
        repoe.y = -109
        
        let dbe = Element(name: "Database")
        _ = repo.add( dbe )
        
        let dbi = pl.add(source: repo, target: dbe)
        dbi?.x = 126
        dbi?.y = -216
        
        
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
