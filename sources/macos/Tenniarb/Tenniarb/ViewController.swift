//
//  ViewController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 26/05/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Cocoa

class ViewController: NSViewController {

    @IBOutlet weak var scene: SceneDrawView!
    
    var elementModel:ElementModel?
    
    override func viewDidLoad() {
        super.viewDidLoad()
    }
    

    override var representedObject: Any? {
        didSet {
        // Update the view, if already loaded.
        }
    }
    
    public func setElementModel(elementModel: ElementModel) {
        self.elementModel = elementModel
        scene.setElementModel(elementModel)
    }
}

