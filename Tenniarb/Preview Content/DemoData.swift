//
//  DemoData.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 20.11.2021.
//  Copyright © 2021 Andrey Sobolev. All rights reserved.
//

import Foundation

let storedValue = """
element "Init screens" {
    element "Simple" {
        styles {
            central {
                font-size 30
            }
            main {
                font-size 20
            }
            child {
                font-size 10
            }
            item {
                shadow -5 -5 5
            }
        }
        item "Central" {
            pos -351.94140625 172.265625
            use-style central
            marker "🎁"
        }
        item "Main Topic 1" {
            pos -216.01718139648438 216.265625
            color orange
            line-style solid
            line-width 1
            use-style main
            marker "♣️"
        }
        link "Central" "Main Topic 1"
        item "Main Topic 2" {
            pos -188.01718139648438 135.265625
            color lightblue
            use-style main
            marker "♥️"
        }
        link "Central" "Main Topic 2"
        item "Main Topic 4" {
            pos -498.0171813964844 116.265625
            color green
            use-style main
            marker "♠️"
        }
        link "Central" "Main Topic 4"
        item "Main Topic 5" {
            pos -460.0171813964844 218.265625
            color purple
            use-style main
            marker "♦️"
        }
        link "Central" "Main Topic 5"
        item "Child Topic 1" {
            pos -103.67890930175781 279.265625
            font-size 12
            use-style child
        }
        link "Main Topic 1" "Child Topic 1"
        item "Child Topic 2" {
            pos -39.67890930175781 209.265625
            font-size 12
            use-style child
        }
        link "Main Topic 1" "Child Topic 2"
        item "Child" {
            pos -510.93670654296875 283.265625
            use-style child
        }
        link "Main Topic 5" "Child"
        item "Child" {
            pos -544.9367065429688 209.265625
            use-style child
        }
        link "Main Topic 5" "Child" {
            target-index 1
        }
        item "Child" {
            pos -561.4884643554688 125.265625
            use-style child
        }
        link "Main Topic 4" "Child" {
            target-index 2
        }
        item "Child" {
            pos -518.4884643554688 76.265625
            use-style child
        }
        link "Main Topic 4" "Child" {
            target-index 3
        }
        item "Child" {
            pos -19.200393676757812 133.265625
            use-style child
        }
        link "Main Topic 2" "Child" {
            target-index 4
        }
        item "Child" {
            pos -95.20039367675781 84.265625
            use-style child
        }
        link "Main Topic 2" "Child" {
            target-index 5
        }
    }
    element "Brainstorm" {
        styles {
            item {
                shadow -5 -5 5
            }
        }
        item "Brainstorm" {
            pos -388.296875 57.37109375
        }
        item "*Generating*
Ideas" {
            pos -262.3843688964844 187.37109375
            display circle
            width 150
            height 150
            line-style dashed
        }
        link "Brainstorm" "*Generating*
Ideas"
        item "*Organizing*
Your Thoughts" {
            pos -249.38436889648438 4.37109375
            display circle
            width 150
            height 150
            color blue
        }
        link "Brainstorm" "*Organizing*
Your Thoughts"
        item "Getting Over
Mental Block" {
            pos -280.3843688964844 -200.62890625
            display circle
            width 150
            height 150
            font-size 18
        }
        link "Brainstorm" "Getting Over
Mental Block"
        item "🚀Write down your goals and problems" {
            pos -107.38436889648438 326.37109375
        }
        link "*Generating*
Ideas" "🚀Write down your goals and problems" {
            pos -36.0 36.0
            layout quad
        }
        item "Do word association" {
            pos -71.38436889648438 262.37109375
        }
        link "*Generating*
Ideas" "Do word association"
        item "central idea" {
            pos 108.18536376953125 291.37109375
            font-size 12
        }
        link "Do word association" "central idea"
        item "any related words" {
            pos 121.18536376953125 258.37109375
            font-size 12
        }
        link "Do word association" "any related words"
        item "Freewrite" {
            pos -33.384368896484375 214.37109375
        }
        link "*Generating*
Ideas" "Freewrite"
        item "Sit down for 15m" {
            pos 67.61602020263672 223.37109375
            font-size 12
        }
        link "Freewrite" "Sit down for 15m"
        item "state your goal" {
            pos 68.61602020263672 191.37109375
            font-size 12
        }
        link "Freewrite" "state your goal" {
            pos -24.0 -19.0
            layout quad
        }
        item "\"I want ...\"" {
            pos 74.61602020263672 156.37109375
            font-size 12
        }
        link "Freewrite" "\"I want ...\"" {
            pos -17.0 -38.0
            layout quad
        }
        item "Draft a list" {
            pos -58.384368896484375 137.37109375
        }
        link "*Generating*
Ideas" "Draft a list" {
            pos 66.0 -5.0
        }
        item "a list of every ideas" {
            pos 66.71368408203125 123.37109375
            font-size 12
        }
        link "Draft a list" "a list of every ideas"
        item "Doodle pictures" {
            pos -86.38436889648438 91.37109375
        }
        link "*Generating*
Ideas" "Doodle pictures" {
            pos -21.0 -44.0
            layout quad
        }
        item "draw as ideas come" {
            pos 65.89434814453125 87.37109375
            font-size 12
        }
        link "Doodle pictures" "draw as ideas come"
        item "Write ideas down on notecards" {
            pos -93.38436889648438 42.37109375
        }
        link "*Organizing*
Your Thoughts" "Write ideas down on notecards" {
            pos -52.0 70.0
            layout quad
        }
        item "Make a mind map" {
            pos -61.0 1.37109375
            marker "🤯"
        }
        link "*Organizing*
Your Thoughts" "Make a mind map" {
            pos -7.0 2.0
        }
        item "Break the problem down into individual steps" {
            pos -68.38436889648438 -37.62890625
        }
        link "*Organizing*
Your Thoughts" "Break the problem down into individual steps" {
            pos -67.0 -9.0
        }
        item "Cube the problem" {
            pos -69.38436889648438 -75.62890625
        }
        link "*Organizing*
Your Thoughts" "Cube the problem" {
            pos -10.0 -28.0
        }
        item "Describe the problem" {
            pos 108.46368408203125 -72.62890625
            font-size 12
        }
        link "Cube the problem" "Describe the problem"
        item "Compare it to other situations" {
            pos 113.46368408203125 -105.62890625
            font-size 12
        }
        link "Cube the problem" "Compare it to other situations" {
            pos -35.0 -27.0
            layout quad
        }
        item "Associate the problem with similar topics" {
            pos 111.46368408203125 -145.62890625
            font-size 12
        }
        link "Cube the problem" "Associate the problem with similar topics" {
            pos -71.0 -38.0
            layout quad
        }
        item "Analyze the problem and its solutions" {
            pos 112.46368408203125 -176.62890625
            font-size 12
        }
        link "Cube the problem" "Analyze the problem and its solutions" {
            pos -89.0 -49.0
            layout quad
        }
        item "Apply it to real world situations" {
            pos 115.46368408203125 -208.62890625
            font-size 12
        }
        link "Cube the problem" "Apply it to real world situations" {
            pos -107.0 -86.0
            layout quad
        }
        item "Argue for and against it" {
            pos 116.46368408203125 -239.62890625
            font-size 12
        }
        link "Cube the problem" "Argue for and against it" {
            pos -132.0 -104.0
            layout quad
        }
        item "Go for a walk" {
            pos -70.38436889648438 -291.62890625
            marker "🚶🏻‍♂️"
        }
        link "Getting Over
Mental Block" "Go for a walk"
        item "Take a break" {
            pos -100.38436889648438 -369.62890625
            marker "☕️"
        }
        link "Getting Over
Mental Block" "Take a break"
        item "help boost creativity" {
            pos 119.38555908203125 -290.62890625
            font-size 12
        }
        link "Go for a walk" "help boost creativity"
        item "get a snack🍿" {
            pos 90.88751220703125 -343.62890625
            font-size 12
        }
        link "Take a break" "get a snack🍿"
        item "read a few news articles 🗞" {
            pos 88.88751220703125 -380.62890625
            font-size 12
        }
        link "Take a break" "read a few news articles 🗞"
        item "make a phone call📞" {
            pos 75.88751220703125 -419.62890625
            font-size 12
        }
        link "Take a break" "make a phone call📞"
        item "Talk to yourself" {
            pos -148.38436889648438 -437.62890625
        }
        link "Getting Over
Mental Block" "Talk to yourself"
        item "Listen to music" {
            pos -335.3843994140625 -393.62890625
            marker "🎧"
        }
        link "Getting Over
Mental Block" "Listen to music"
    }
    element "How to" {
        styles {
            item {
                shadow -5 -5
            }
        }
        item "How to create a mind map" {
            pos -180.328125 60.87109375
            font-size 22
            color green
        }
        item "Use color" {
            pos -110.6587905883789 -19.12890625
            color orange
            marker "🔥"
        }
        link "How to create a mind map" "Use color"
        item "Group your ideas" {
            pos -124.14570617675781 -110.12890625
            body "by using color"
            color orange-100
        }
        link "Use color" "Group your ideas"
        item "Share it" {
            pos -299.3247985839844 13.87109375
            color light-blue-100
            marker "🌎"
        }
        link "How to create a mind map" "Share it"
        item "Go deeper" {
            pos -306.3247985839844 131.87109375
            color green-200
            marker "⌚️"
        }
        link "How to create a mind map" "Go deeper"
        item "Every node on mindmap" {
            pos -438.32440185546875 185.87109375
            body "Could be its own mindmap"
            color green-300
        }
        link "Go deeper" "Every node on mindmap"
        item "Nobody is perfect" {
            pos -144.32479858398438 158.87109375
            color blue-400
            marker "-😱-"
        }
        link "How to create a mind map" "Nobody is perfect"
        item "Tidy up later" {
            pos -14.696483612060547 242.87109375
            font-size 12
            color blue-500
        }
        link "Nobody is perfect" "Tidy up later"
        item "Don't focus
on perfection" {
            pos -172.6964874267578 234.87109375
            font-size 12
            color blue-500
        }
        link "Nobody is perfect" "Don't focus
on perfection"
        item "Let's your ideas
Explode!🧨" {
            pos -98.69648742675781 284.87109375
            font-size 12
            color blue-500
        }
        link "Nobody is perfect" "Let's your ideas
Explode!🧨"
        item "Break the \"on the page\"" {
            pos 112.7835922241211 105.87109375
            color red-200
            body "mentality"
            marker "✂️"
        }
        link "How to create a mind map" "Break the \"on the page\""
        item "Your brain isn't confined" {
            pos 154.2205047607422 178.87109375
            body "to one page"
            color red-300
        }
        link "Break the \"on the page\"" "Your brain isn't confined"
        item "So your mindmaps" {
            pos 203.8322296142578 254.87109375
            body "shouldn't be either."
            color red-400
        }
        link "Your brain isn't confined" "So your mindmaps"
        item "Don't take sides" {
            pos 123.7835922241211 4.87109375
            color purple-100
        }
        link "How to create a mind map" "Don't take sides"
        item "Embrace both sides  of your brain" {
            pos 103.86112976074219 -53.12890625
            color purple-200
        }
        link "Don't take sides" "Embrace both sides  of your brain"
        item "The Creative" {
            pos 127.19257354736328 -110.12890625
            color purple-400
        }
        link "Embrace both sides  of your brain" "The Creative"
        item "The Analytical" {
            pos 291.19256591796875 -111.12890625
            color purple-400
        }
        link "Embrace both sides  of your brain" "The Analytical"
        item "When you finish" {
            pos -407.15740966796875 -63.12890625
            body "Share it with your Collegues."
            color light-blue-200
        }
        link "Share it" "When you finish"
        item "This will help you." {
            pos -419.0 -148.0
            body "Get a fresh perspective."
            color light-blue-300
        }
        link "When you finish" "This will help you."
    }
    element "Task Planing" {
        define ${
    function days_width() {
        if( days != undefined) {
            return days * 15
        }
        return -1
    }
}
        styles {
            item {
                shadow -5 -5
            }
        }
        item "April" {
            pos -669.54248046875 317.0
            days 12
            width ${days_width()}
            color orange
            month 1
        }
        item "May" {
            pos -485.54248046875 317.0
            days 15
            width ${days_width()}
            color orange
            month 1
        }
        item "Person2" {
            pos -764.0 182.0
            color green
            body "${sum('Person2','days')}"
            kind Person
        }
        item "Task5" {
            pos -155.0 252.0
            body %{Metadata}
            days 5
            width ${days_width()}
            kind Task
            Person1 1
        }
        item "Task4" {
            pos -230.0 252.0
            body %{Parser}
            days 4
            width ${days_width()}
            kind Task
            Person1 1
        }
        item "Task16" {
            pos -564.0 -50.0
            body "Come CSS fixes"
            days 10
            width ${days_width()}
            color yellow
            kind Task
        }
        item "Task20" {
            pos -465.0 -209.0
            body %{Some unplaned work}
            days 17
            width ${days_width()}
            color Grey
            kind Task
        }
        item "Person3" {
            pos -764.0 111.0
            color green
            body "${sum('Person3','days')}"
            kind Person
        }
        item "Task17" {
            pos -404.0 -50.0
            body "Update process"
            days 7
            width ${days_width()}
            kind Task
        }
        item "Task15" {
            pos -628.0 -50.0
            body "Post processing"
            days 3
            width ${days_width()}
            color orange
            kind Task
        }
        item "Task6" {
            pos -663.0 111.0
            body %{View model}
            days 5
            width ${days_width()}
            color orange
            kind Task
            Person3 1
        }
        item "Task3" {
            pos -321.0 252.0
            body %{Cleanup}
            days 5
            width ${days_width()}
            display rect
            kind Task
            Person1 1
        }
        item "Task9" {
            pos -220.0 111.0
            body %{Check network}
            days 7
            width ${days_width()}
            kind Task
            Person3 1
        }
        item "Person4" {
            pos -764.0 30.0
            color green
            body "${sum('Denis','days')}"
            kind Person
        }
        item "Task7" {
            pos -580.0 111.0
            body %{Structure parser}
            days 12
            width ${days_width()}
            kind Task
            Person3 1
        }
        item "Person1" {
            pos -764.0 252.0
            color green
            body "${sum('Person1','days')}"
            kind Person
        }
        item "Task13" {
            pos -156.0 30.0
            body %{Write tests}
            days 6
            width ${days_width()}
            kind Task
        }
        item "Task8" {
            pos -383.0 111.0
            body %{Integration}
            days 10
            width ${days_width()}
            kind Task
            Person3 1
        }
        item "Person5" {
            pos -764.0 -50.0
            body "${sum('Artyom','days')}"
            color green
            kind Person
        }
        item "Task10" {
            pos -104.0 111.0
            body %{Profile}
            days 8
            width ${days_width()}
            kind Task
            Person3 1
        }
        item "Task14" {
            pos -654.0 -50.0
            body "Update date"
            days 1
            width ${days_width()}
            color orange
            kind Task
        }
        item "Task1 - Phase1" {
            pos -663.0 182.0
            body %{Do something 2}
            days 19
            width ${days_width()}
            color orange
            kind Task
            Person2 1
        }
        item "June" {
            pos -255.54248046875 317.0
            days 20
            width ${days_width()}
            color orange
            month 1
        }
        item "12" {
            pos -593.9425048828125 366.736328125
        }
        link "April" "12" {
            width ${days_width()}
        }
        item "15" {
            pos -391.9424743652344 367.736328125
        }
        link "May" "15" {
            width ${days_width()}
        }
        item "20" {
            pos -124.94247436523438 369.736328125
        }
        link "June" "20" {
            width ${days_width()}
        }
        item "Task18" {
            pos -286.0 -50.0
            body "Write help"
            days 9
            width ${days_width()}
            kind Task
        }
        item "month_totals" {
            pos 58.924072265625 75.23068237304688
            title "Month Total"
            team_total ${sum('month','days')*5}
            body %{
    Month: ${sum('month','days')}
    Team: ${team_total}
}
            font-size 16
        }
        item "Task19" {
            pos -642.0 -206.0
            body %{Some extra work}
            days 10
            width ${days_width()}
            color grey
            kind Task
        }
        item "Task12" {
            pos -258.0 30.0
            body %{Benchmark}
            days 6
            width ${days_width()}
            kind Task
        }
        item "Person Total" {
            pos 77.924072265625 264.2306823730469
            body %{${sum('person','body')}}
            font-size 16
        }
        item "Person Total" {
            pos 48.924072265625 174.23068237304688
            body %{
   Person assignee * 1.5: ${sum('person','body')*1.5}
   Person assignee * 1.3: ${sum('person','body')*1.3}
}
            font-size 16
        }
        item "Task1/UI - Phase2" {
            pos -571.0 30.0
            body %{Do something}
            days 20
            width ${days_width()}
            color orange
            kind Task
        }
        item "Task2/Core - Phase2" {
            pos -666.0 252.0
            body %{Do something 1}
            days 22
            width ${days_width()}
            color orange
            kind Task
            Person1 1
        }
        item "Task2/UI - Phase2" {
            pos -370.0 182.0
            body %{do some UI}
            days 19
            width ${days_width()}
            color orange
            kind Task
            Person2 1
        }
        item "Task11" {
            pos -661.3040771484375 30.0
            body "Write commands"
            days 5
            width ${days_width()}
            kind Task
        }
        item "Untitled 1" {
            pos -74.0015869140625 -107.3583984375
            body "${Math.floor(items.Person1.pos.x) } ${Math.floor(items.Person1.pos.y)}"
        }
    }
}
"""

func loadTestDocument() -> Element {
        let now = Date()

        let parser = TennParser()
        let node = parser.parse(storedValue)

        if parser.errors.hasErrors() {
            return Element(name: "Failed element")
        }

        let elementModel = ElementModel.parseTenn(node: node)
        Swift.debugPrint("Loaded done")
        Swift.debugPrint("Elapsed parse \(Date().timeIntervalSince(now))")
        return elementModel
}
