//
//  MainView.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 23.06.2020.
//  Copyright © 2020 Andrey Sobolev. All rights reserved.
//

import Foundation
import SwiftUI

struct TreeOutline: View {
    @State
    var elements: [Element]
    
    var body: some View {
        VStack {
            ForEach(0 ..< self.elements.count) {itmIndex in
                VStack {
                    Text(self.elements[itmIndex].name)
                    TreeOutline(elements:self.elements[itmIndex].elements)
                }.scaledToFit()
            }
        }.listStyle(SidebarListStyle())
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}
struct SceneDetailsView: View {
    var body: some View {
        Text("some data")
            .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

struct MainView: View {
    @State
    var scene: Element
    var body: some View {
        NavigationView {
            TreeOutline(elements: scene.elements)
            SceneDetailsView()
        }
        .navigationTitle(scene.name)
        .navigationSubtitle("Test")
        .toolbar {
            ToolbarItem {
                Text("Test1")
            }
            ToolbarItem {
                Text("Test2")
            }
        }
    }
}



struct MainView_Previews: PreviewProvider {
    static var scene: Element = loadTestDocument()
    static var previews: some View {
        MainView(scene: scene).frame(width: 500, height: 500)
    }
}

func loadTestDocument() -> Element {
    do {
        let url = URL(fileURLWithPath: "/Users/haiodo/Develop/private/tenniarb/sources/macos/Tenniarb/docs/Example.tenn")
        let storedValue = try String(contentsOf: url, encoding: String.Encoding.utf8)
        
        let now = Date()
        
        let parser = TennParser()
        let node = parser.parse(storedValue)
        
        if parser.errors.hasErrors() {
            return Element(name: "Failed element")
        }
        
        let elementModel = ElementModel.parseTenn(node: node)
        
        Swift.debugPrint("Elapsed parse \(Date().timeIntervalSince(now))")
        return elementModel
    }
    catch {
        return Element(name: "Failed element")
    }
}
