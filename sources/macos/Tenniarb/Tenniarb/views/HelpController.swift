//
//  HelpController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 10.07.2019.
//  Copyright © 2019 Andrey Sobolev. All rights reserved.
//

import Foundation
import Cocoa
import WebKit

class HelpController: NSViewController  {
    @IBOutlet weak var webView: WKWebView!
    override func viewDidLoad() {
        super.viewDidLoad()
        let filePath = Bundle.main.path(forResource: "readme", ofType: "html")
        let request = URLRequest(url: URL(fileURLWithPath: filePath!))
        webView.load(request)
    }
}
