////
////  MainView.swift
////  Tenniarb
////
////  Created by Andrey Sobolev on 23.06.2020.
////  Copyright © 2020 Andrey Sobolev. All rights reserved.
////
//
//import Foundation
//import SwiftUI
//
//struct Item: View {
//    @State
//    var item: DiagramItem
//    var body: some View {
//        ZStack {
//            Text(item.name)
//            RoundedRectangle(cornerRadius: 8)
//                        .stroke(style: StrokeStyle(lineWidth: 1, lineCap: .square, dash: [1], dashPhase: 2))
//                        .foregroundColor(.purple)
//        }.frame(width: item.width, height: item.height).position(x: item.x + 250, y: item.y+250)
//    }
//}
//
//struct MainView: View {
//    @State
//    var scene: DrawableScene
//    var body: some View {
//        ZStack {
//            ForEach(self.scene.items, id: \.id, content: { child in
//                Item(item:child)
//            })
//        }
//    }
//}
//
//
//
//struct MainView_Previews: PreviewProvider {
//    static var scene: Element = loadTestDocument()
//    static var previews: some View {
//        MainView(scene: scene).frame(width: 800, height: 600)
//    }
//}
//
//func loadTestDocument() -> DrawableScene {
//    do {
//        let url = URL(fileURLWithPath: "/Users/haiodo/Develop/private/tenniarb/sources/macos/Tenniarb/docs/Example.tenn")
//        let storedValue = try String(contentsOf: url, encoding: String.Encoding.utf8)
//        
//        let now = Date()
//        
//        let parser = TennParser()
//        let node = parser.parse(storedValue)
//        
//        if parser.errors.hasErrors() {
//            return Element(name: "Failed element")
//        }
//        
//        let elementModel = ElementModel.parseTenn(node: node)
//        Swift.debugPrint(elementModel.elements)
//        Swift.debugPrint("Elapsed parse \(Date().timeIntervalSince(now))")
//        let el = elementModel.elements[0].elements[2].elements[0].elements[0]
//        
//        let store = ElementModelStore(elementModel)
//        let scene = DrawableScene(el, darkMode: false, executionContext: store.executionContext, scaleFactor: 1)
//        return scene
//    }
//    catch {
//        return Element(name: "Failed element")
//    }
//}
