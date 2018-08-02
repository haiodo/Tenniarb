//
//  SyncExtension.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 25/07/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
//

import Foundation

struct SyncElement: Codable {
    let name: String
    let description: String?
    let items: [SyncItem]?
    let edges: [SyncItem]?
}

struct SyncItem: Codable {
    let kind: String
    let name: String
    let id: String
    let pos: SyncPos
    let description: String?
    let properties: [[String]]?
    let source: String?
    let target: String?
}
struct SyncPos: Codable {
    let x: CGFloat
    let y: CGFloat
}

extension TennNode {
    func toSync() -> [String] {
        if self.count > 0 {
            return self.children!.map {(itm) in itm.toStr(0, true).replacingOccurrences(of: "\n", with: "\\n")}
        }
        else {
            return []
        }
    }
}

extension Element {
    
    func toSyncJson() -> String {
        let encoder = JSONEncoder()
        encoder.outputFormatting = .prettyPrinted
        encoder.dataEncodingStrategy = .base64
        
        let syncModel = self.toSync()
        let data = try! encoder.encode(syncModel)
        let txtValue = String( data: data, encoding: .utf8)!
        return txtValue
    }
    func toSync() -> SyncElement {
        
        var items: [SyncItem]? = []
        var edges: [SyncItem]? = []
        
        self.items.forEach {
            (itm) in
            var source: String? = nil
            var target: String? = nil
            var lnkKind = false
            if let lnk = itm as? LinkItem, let sourceItm = lnk.source, let targetItm = lnk.target {
                source = sourceItm.id.uuidString
                target = targetItm.id.uuidString
                lnkKind = true
            }
            let itm = SyncItem(
                kind: itm.kind.commandName,
                name: itm.name.replacingOccurrences(of: "\n", with: "\\n"),
                id: itm.id.uuidString,
                pos: SyncPos(x: itm.x,
                             y: itm.y),
                description: itm.description?.replacingOccurrences(of: "\n", with: "\\n"),
                properties: itm.properties.count == 0 ? nil : itm.properties.map { (prop) in prop.toSync() },
                source: source,
                target: target
            )
            if lnkKind {
                edges?.append(itm)
            }
            else {
                items?.append(itm)
            }
            
        }
        if items?.count == 0 {
            items = nil
        }
        if edges?.count == 0 {
            edges = nil
        }
 
        let result = SyncElement(
            name:self.name,
            description: self.description,
            items: items,
            edges: edges
        )
        
        return result
    }
}

func runScriptWithDictionary(arguments:[String], content: String) -> String? {
    let outPipe = Pipe()
    let errPipe = Pipe()
    let inPipe = Pipe()
    
    let task = Process()
    task.launchPath = arguments[0]
    task.arguments = arguments
    task.standardInput = inPipe
    task.standardOutput = outPipe
    task.standardError = errPipe
    task.launch()
    
    inPipe.fileHandleForWriting.write(content.data(using: String.Encoding.utf8)!)
    
    let data = outPipe.fileHandleForReading.readDataToEndOfFile()
    task.waitUntilExit()
    
    let exitCode = task.terminationStatus
    if (exitCode != 0) {
        print("ERROR: \(exitCode)")
        return nil
    }
    
    return String(data: data, encoding: String.Encoding.utf8)
}
