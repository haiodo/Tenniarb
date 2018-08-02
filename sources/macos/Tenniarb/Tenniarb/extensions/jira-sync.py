from jira import JIRA
from getpass import getpass
import json
import uuid

sync_data = """
{
  "name" : "Execution plan",
  "items" : [
    {
      "id" : "953FA461-2BF7-4610-872F-E6A6B3A04103",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "RM-13953\\nDataFlow",
      "pos" : {
        "x" : -468,
        "y" : 91
      }
    },
    {
      "id" : "2C098078-F722-485E-B85E-2C1F5F8C2772",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "RM-13954\\nUI fix at WTE",
      "pos" : {
        "x" : -155,
        "y" : 92
      }
    },
    {
      "id" : "7BAECFBB-44A8-4A3F-B5AC-4693D5F39CFB",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "RM-13957\\nSession form",
      "pos" : {
        "x" : -396.8671875,
        "y" : 91
      }
    },
    {
      "id" : "55D94F4C-A362-4C27-BB50-B1C39137A092",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ]
      ],
      "name" : "RM-13964\\nAgent Requirements",
      "pos" : {
        "x" : 710,
        "y" : 104
      }
    },
    {
      "id" : "DE3CD03E-6322-42C6-8212-AB526F1C8E2B",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "lightgreen"
        ]
      ],
      "name" : "RM-13965\\nPrompts form",
      "pos" : {
        "x" : 174,
        "y" : 102
      }
    },
    {
      "id" : "C8AB9DA4-A7C5-4FC1-8AE5-153DE9A00DDF",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ]
      ],
      "name" : "RM-13969\\nUndo\/Redo",
      "pos" : {
        "x" : 944,
        "y" : 85
      }
    },
    {
      "id" : "8C0E976E-B314-461C-A786-8B0DACD1D851",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "RM-13092\\nAssociate\\nsession",
      "pos" : {
        "x" : -227,
        "y" : 82
      }
    },
    {
      "id" : "D2CBDBCA-1AAC-4E62-BCB5-DCF7187FBCD1",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ]
      ],
      "name" : "RM-13767\\nDemo\\nFinal",
      "pos" : {
        "x" : 1126,
        "y" : 95
      }
    },
    {
      "id" : "A82E2422-3137-4DCF-B665-77117E2654AF",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "lightgreen"
        ]
      ],
      "name" : "RM-13096\\nTelnet Session",
      "pos" : {
        "x" : 50,
        "y" : 102
      }
    },
    {
      "id" : "1E1355EE-AA59-4E4E-9B3A-D4B5D9A37DCF",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "RM-13746\\nSSH Session",
      "pos" : {
        "x" : -312,
        "y" : 91
      }
    },
    {
      "id" : "2B935C0E-FD8B-4E9A-84EA-8D0765A1DEFD",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "RM-13767\\nDemo-part1",
      "pos" : {
        "x" : -66.74609375,
        "y" : 118
      }
    },
    {
      "id" : "1D3A65C5-7454-499D-B110-79C4EF4E06C7",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "lightgreen"
        ]
      ],
      "name" : "RM-13765\\nDev testing",
      "pos" : {
        "x" : -65,
        "y" : 76
      }
    },
    {
      "id" : "1F5B7459-3D8C-441B-98D9-308BE8229298",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ]
      ],
      "name" : "RM-13765\\nDev testing",
      "pos" : {
        "x" : 859.32421875,
        "y" : 99
      }
    },
    {
      "id" : "3426074E-3487-47E7-B208-A36CA6EDA3EC",
      "kind" : "item",
      "name" : "July 2",
      "pos" : {
        "x" : -561.0576171875,
        "y" : 186
      }
    },
    {
      "id" : "82E032AC-1438-43C3-8DFF-1179C7F153B4",
      "kind" : "item",
      "properties" : [
        [
          "color",
          "yellow"
        ],
        [
          "font-size",
          "20"
        ]
      ],
      "name" : "Denis",
      "pos" : {
        "x" : -610,
        "y" : 117
      }
    },
    {
      "id" : "4A5D8A39-4C0E-4413-8EDF-676DB089E858",
      "kind" : "item",
      "properties" : [
        [
          "color",
          "yellow"
        ]
      ],
      "name" : "Andrey",
      "pos" : {
        "x" : -610,
        "y" : -38
      }
    },
    {
      "id" : "2679C912-86E1-4E3D-ABAF-3AF73E810416",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "RM-13961\\nJS Protobuf",
      "pos" : {
        "x" : -463.298828125,
        "y" : 20
      }
    },
    {
      "id" : "6AA1F36F-D360-429C-92A7-7C7512C20620",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "lightgreen"
        ]
      ],
      "name" : "RM-13740\\nTerminalManager",
      "pos" : {
        "x" : -381.443359375,
        "y" : 19
      }
    },
    {
      "id" : "66D026CA-9EF7-4BE8-8439-05F72328C07F",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "RM-13955\\nEmbeddXterm",
      "pos" : {
        "x" : -270.955078125,
        "y" : 20
      }
    },
    {
      "id" : "B439BB52-41C2-4341-8044-E2DD386246C9",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "RM-13963\\nMock server",
      "pos" : {
        "x" : -178.23046875,
        "y" : 18
      }
    },
    {
      "id" : "AD2CBC50-B7D2-4834-8D14-FCDDEA1DF82B",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "SPRING-3222\\nRH-Websockets",
      "pos" : {
        "x" : -94.443359375,
        "y" : 20
      }
    },
    {
      "id" : "D946993B-0D3D-4913-A4E8-C8265DFA4C11",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "SPRING-3225\\nResponseHandler-Demo",
      "pos" : {
        "x" : -237.994140625,
        "y" : -68
      }
    },
    {
      "id" : "F3C2A4AC-B28C-4942-9FB5-A7DA30B9FDE1",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "SPRING-3215\\nSLC Protocol update",
      "pos" : {
        "x" : -464.955078125,
        "y" : -23
      }
    },
    {
      "id" : "8C22721F-82B5-4B65-9DC0-D09C1401EE45",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "ITEST-15248\\nSLC TerminalData",
      "pos" : {
        "x" : -464,
        "y" : -66
      }
    },
    {
      "id" : "45900103-C33A-493F-B098-ED5D6F62C5A9",
      "kind" : "item",
      "properties" : [
        [
          "color",
          "yellow"
        ]
      ],
      "name" : "Viktoria",
      "pos" : {
        "x" : -610,
        "y" : -131
      }
    },
    {
      "id" : "5BCB6C6D-6849-49F6-9397-8F53356A8A8F",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "SPRING-3292\\nKafka in Response\\nHandler",
      "pos" : {
        "x" : 20.697265625,
        "y" : 7
      }
    },
    {
      "id" : "75BDD286-D579-4E47-970A-1B2D798054CD",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "ITEST-15249\\nSLC Agent Support\\nNested Step Details",
      "pos" : {
        "x" : 292.5546875,
        "y" : 4
      }
    },
    {
      "id" : "855142BB-DD3B-4280-9E17-1E2C35190255",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ],
        [
          "draw",
          "{\\n    rect 0 0 600 700\\n}"
        ]
      ],
      "name" : "RM-13955\\nEmbeddXterm -Widget + Resizing",
      "pos" : {
        "x" : -321.23046875,
        "y" : -131
      }
    },
    {
      "id" : "0CE89336-312D-4036-8A03-6CE766CB36E6",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "RM-13956 - UI for \\nTerminalManager",
      "pos" : {
        "x" : -332.23046875,
        "y" : -25
      }
    },
    {
      "id" : "C83E7D87-C8CA-44BE-AF0B-F33F2E485744",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "RM-13956 - UI for \\nTerminalManager",
      "pos" : {
        "x" : -110.7890625,
        "y" : -131
      }
    },
    {
      "id" : "0A419833-6D5B-4030-8D15-FA4B3337F4ED",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ]
      ],
      "name" : "SPRING-3294\\nResponse Handler\\nNon session commands",
      "pos" : {
        "x" : 857.478515625,
        "y" : -220
      }
    },
    {
      "id" : "AC1C554E-32AB-40CA-9422-E61A1E533D6B",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ]
      ],
      "name" : "RM-13514\\nWebIDE\\nLicensing Emforcement",
      "pos" : {
        "x" : 1064.708984375,
        "y" : -143
      }
    },
    {
      "id" : "82760D0B-1FA9-414A-9BF5-5914C3884EBF",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "ITEST-15163\\nQuickcall-selector UI",
      "pos" : {
        "x" : 24.796875,
        "y" : -129
      }
    },
    {
      "id" : "399F7B49-8BA8-43B0-9F67-450CF00E7DEE",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "ITEST-15232\\nQuickcall calling",
      "pos" : {
        "x" : 160.94140625,
        "y" : -130
      }
    },
    {
      "id" : "CF2242DC-5A02-4E0D-B7A2-CECFD2FAB4D5",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "lightgreen"
        ]
      ],
      "name" : "ITEST-15231\\nQuickcall History",
      "pos" : {
        "x" : 502.51171875,
        "y" : -130
      }
    },
    {
      "id" : "4DD90AFC-DF85-4B5B-82C7-630424AA1259",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : -650,
        "y" : 60
      }
    },
    {
      "id" : "F45B9D19-960C-4A5D-8A08-9D336C7902CB",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : 1600,
        "y" : 60
      }
    },
    {
      "id" : "DE962CE9-E19C-4F72-9D58-D4C777C642BF",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : -650,
        "y" : -90
      }
    },
    {
      "id" : "3068C334-782C-4789-859F-90A5305A678D",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : 1600,
        "y" : -90
      }
    },
    {
      "id" : "1451151F-F916-4334-92E2-3BCBFC14D8FE",
      "kind" : "item",
      "name" : "July 16",
      "pos" : {
        "x" : -417.18359375,
        "y" : -122
      }
    },
    {
      "id" : "F819AC8E-493D-4CBE-951D-E7B866A99929",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : -650,
        "y" : -160
      }
    },
    {
      "id" : "3D05C6C8-A98E-416C-80D3-5053355F5F25",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : 1600,
        "y" : -160
      }
    },
    {
      "id" : "5F780035-A51B-4EEC-BE7B-EEEE5D51506B",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ],
        [
          "width",
          "400"
        ]
      ],
      "name" : "SPRING-3236\\nSLC ExecutionService ",
      "pos" : {
        "x" : -427.720703125,
        "y" : -195
      }
    },
    {
      "id" : "6D0CB27A-3F36-4C3A-B261-B35E1146E33D",
      "kind" : "item",
      "properties" : [
        [
          "color",
          "yellow"
        ]
      ],
      "name" : "Stepan",
      "pos" : {
        "x" : -610,
        "y" : -202
      }
    },
    {
      "id" : "2CFF6D6E-2175-4546-9B0F-96139FA5367A",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "SPRING-3295\\nSecurity",
      "pos" : {
        "x" : -78.994140625,
        "y" : -237
      }
    },
    {
      "id" : "1DC8C46A-8214-47D6-A31B-75D8C39DCC96",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ]
      ],
      "name" : "SPRING-2606\\nSLC Licensing Enforcement",
      "pos" : {
        "x" : 514.404296875,
        "y" : -198
      }
    },
    {
      "id" : "4F20672E-F9D2-47D6-9B0D-0502E427B715",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ]
      ],
      "name" : "SPRING-3122\\nSLC Capture Database",
      "pos" : {
        "x" : 499.70703125,
        "y" : -283
      }
    },
    {
      "id" : "2D776101-3D43-4B0B-8614-A479FFB5677C",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ]
      ],
      "name" : "SPRING-3219\\nSLC Capture Collector",
      "pos" : {
        "x" : 643.76953125,
        "y" : -284
      }
    },
    {
      "id" : "3EFB55F1-E73D-458E-BD2A-9AE0F253836D",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ]
      ],
      "name" : "RM-13101\\nCapture History per Reservation",
      "pos" : {
        "x" : 1223.9609375,
        "y" : -125
      }
    },
    {
      "id" : "4D67EF7C-790B-4D8D-BB4E-F71BA6A5D5C5",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : -650,
        "y" : -250
      }
    },
    {
      "id" : "89470140-0A5E-434B-97AF-9083C996BED1",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : 1600,
        "y" : -250
      }
    },
    {
      "id" : "2211BBBB-270B-4C18-B66C-859E9F65272C",
      "kind" : "item",
      "properties" : [
        [
          "color",
          "yellow"
        ]
      ],
      "name" : "Ruslan",
      "pos" : {
        "x" : -610,
        "y" : -293
      }
    },
    {
      "id" : "102EA4C1-05AD-41EB-A69D-53A6E010A44E",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ],
        [
          "width",
          "200"
        ]
      ],
      "name" : "ITEST-14218\\nIntegration with Kafka ES",
      "pos" : {
        "x" : -430.720703125,
        "y" : -299
      }
    },
    {
      "id" : "96DC62F8-FD00-4A32-8749-6994245F2DDB",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ]
      ],
      "name" : "RM-13497\\nSLC Agent\\nManagement",
      "pos" : {
        "x" : 732.091796875,
        "y" : -145
      }
    },
    {
      "id" : "BADB9AFA-D0B3-4A2D-B639-F82183F97F02",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "ITEST-15144\\nVelocity sessions",
      "pos" : {
        "x" : -349.892578125,
        "y" : -66
      }
    },
    {
      "id" : "A1BF727D-BE39-43C4-AB67-2408213B14AB",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ]
      ],
      "name" : "SPRING-3216\\nSLC Performance Tuning",
      "pos" : {
        "x" : 1005.107421875,
        "y" : -208
      }
    },
    {
      "id" : "521C7175-48C0-4045-954B-28F41B35E000",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : -650,
        "y" : -325
      }
    },
    {
      "id" : "9EB4725C-580E-4E2E-A4C4-57F6206CCDB6",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : 1600,
        "y" : -325
      }
    },
    {
      "id" : "D1A01FA2-9AE9-4579-B802-8795EFB01670",
      "kind" : "item",
      "properties" : [
        [
          "color",
          "yellow"
        ]
      ],
      "name" : "+Julia",
      "pos" : {
        "x" : -610,
        "y" : -360
      }
    },
    {
      "id" : "26320B16-EA71-4985-B8B9-BE7B4C502C44",
      "kind" : "item",
      "properties" : [
        [
          "color",
          "yellow"
        ]
      ],
      "name" : "Evgeny",
      "pos" : {
        "x" : -610,
        "y" : -438
      }
    },
    {
      "id" : "C61332AE-8A72-4D8C-90DF-8D849AD284C2",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ],
        [
          "width",
          "300"
        ]
      ],
      "name" : "ITEST-14420\\nSelenium IDE for Chrome",
      "pos" : {
        "x" : -291.720703125,
        "y" : -440
      }
    },
    {
      "id" : "E1EA3E50-FAFF-417F-A325-0A41E0D12952",
      "kind" : "item",
      "name" : "July 11",
      "pos" : {
        "x" : -326.3095703125,
        "y" : -521
      }
    },
    {
      "id" : "711BC4E1-7063-4E70-AA5D-AEEA21D8A54A",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "SPRING-3236\\n Demo",
      "pos" : {
        "x" : 30.279296875,
        "y" : -200
      }
    },
    {
      "id" : "F39AE8F5-D8B4-4925-88A8-BEC2F8D3E1FB",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "ITEST-15264\\nDemo",
      "pos" : {
        "x" : 195.537109375,
        "y" : -441
      }
    },
    {
      "id" : "062B7115-A86F-4794-9986-B8451FD2D39F",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "ITEST-15139\\nSLC Agent Support-Demo",
      "pos" : {
        "x" : -212.892578125,
        "y" : -24
      }
    },
    {
      "id" : "50EDEAD4-B868-4EED-89EA-79424ADF4F6D",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "lightgreen"
        ]
      ],
      "name" : "ITEST-14772\\nresponseLine() Query",
      "pos" : {
        "x" : 291.794921875,
        "y" : -443
      }
    },
    {
      "id" : "88F454D1-E575-43C9-B403-D1660FA84200",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "lightgreen"
        ]
      ],
      "name" : "ITEST-15270\\nTestingTool enhancement",
      "pos" : {
        "x" : -185.3798828125,
        "y" : -372
      }
    },
    {
      "id" : "522BF915-B55E-41D9-88D7-6B242E133BC3",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "red"
        ]
      ],
      "name" : "ITEST-15190 - Kafka demo",
      "pos" : {
        "x" : 500.4541015625,
        "y" : -311
      }
    },
    {
      "id" : "CD0444C9-9489-4BE6-B897-C9262C26E92A",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "ITEST-15188\\nKafka dev testing",
      "pos" : {
        "x" : 26.2724609375,
        "y" : -299
      }
    },
    {
      "id" : "2A8A7ACF-535B-4401-9346-69B79DEDE39F",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "yellow"
        ]
      ],
      "name" : "Delay +2 days",
      "pos" : {
        "x" : -113.845703125,
        "y" : 168
      }
    },
    {
      "id" : "D0B3A748-0169-411E-9834-97F1B6DDAD34",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "lightgreen"
        ]
      ],
      "name" : "SPRING-3227\\n ES Dev testing",
      "pos" : {
        "x" : 280.091796875,
        "y" : -202
      }
    },
    {
      "id" : "6465C0D8-1351-4156-B318-BE6F5D1B013B",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "SPRING-3236\\nSLC ExecutionService ",
      "pos" : {
        "x" : 134.279296875,
        "y" : -201
      }
    },
    {
      "id" : "18513366-9E6F-40FB-BC68-6F427596AA7A",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : 7,
        "y" : 200
      }
    },
    {
      "id" : "3BD28E50-7D53-44B1-AF82-162A665D1083",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : 7,
        "y" : -500
      }
    },
    {
      "id" : "CE88F4BC-AF65-4DF7-8898-FEBF55C0F35F",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : 485,
        "y" : 200
      }
    },
    {
      "id" : "7DE4BE2A-7E89-43C9-9998-D0DE38AD2C89",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : 830,
        "y" : 200
      }
    },
    {
      "id" : "29E4ADC7-D000-4697-B9FF-B34297ABAA40",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : 489,
        "y" : -500
      }
    },
    {
      "id" : "CD959844-436B-4188-9C3B-508E156E8578",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : 828,
        "y" : -500
      }
    },
    {
      "id" : "86C79586-70F1-4CC8-B071-4A03E979AB79",
      "kind" : "item",
      "name" : "23-27",
      "pos" : {
        "x" : 218,
        "y" : 173
      }
    },
    {
      "id" : "0A668F43-9CF5-41F6-B35E-F2F3E8ADE978",
      "kind" : "item",
      "name" : "30-3 Aug-current",
      "pos" : {
        "x" : 601.623046875,
        "y" : 167
      }
    },
    {
      "id" : "B9328689-04B6-401E-A34E-41CC2673E010",
      "kind" : "item",
      "name" : "6-10 Aug",
      "pos" : {
        "x" : 983.298828125,
        "y" : 176
      }
    },
    {
      "id" : "5843104B-F527-489B-BD3D-C772B886A9EE",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ]
      ],
      "name" : "SPRING-3270\\nSLC Capture Database\\nGrowth Management",
      "pos" : {
        "x" : 846.76953125,
        "y" : -306
      }
    },
    {
      "id" : "4F64DCAC-8689-4E7D-883C-30495452B554",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "ITEST-15251\\nSLC CaptureMessage",
      "pos" : {
        "x" : 312.76953125,
        "y" : -304
      }
    },
    {
      "id" : "F1D19D2B-2B61-41A9-88DB-66C2A7C255DC",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "yellow"
        ],
        [
          "height",
          "70"
        ],
        [
          "width",
          "430"
        ]
      ],
      "name" : "Vacation from 3 Aug to 13 Aug",
      "pos" : {
        "x" : 808.708984375,
        "y" : -9
      }
    },
    {
      "id" : "2FFEE475-60F2-4231-8CED-CC3DCDDF0096",
      "kind" : "item",
      "name" : "13-17",
      "pos" : {
        "x" : 1394.794921875,
        "y" : 172
      }
    },
    {
      "id" : "616FB269-7D4B-4320-8E87-16A7697CE36B",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : 1211,
        "y" : 200
      }
    },
    {
      "id" : "9212173F-77CF-42B1-B2F7-EBEC1E45346D",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : 1209,
        "y" : -500
      }
    },
    {
      "id" : "97D7519E-BB97-4D64-8558-492E8B032855",
      "kind" : "item",
      "name" : "Integration week",
      "pos" : {
        "x" : 958.794921875,
        "y" : 135
      }
    },
    {
      "id" : "4CD8CA95-AADA-40D1-BBCE-55F7205DF741",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "yellow"
        ],
        [
          "height",
          "50"
        ],
        [
          "width",
          "560"
        ]
      ],
      "name" : "Vacation from 1 Aug to 15 Aug",
      "pos" : {
        "x" : 716.708984375,
        "y" : -456
      }
    },
    {
      "id" : "074D2EA2-DF36-4706-A020-9102196D4318",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ]
      ],
      "name" : "RM-13740\\nTerminalManager",
      "pos" : {
        "x" : 615.044921875,
        "y" : -129
      }
    },
    {
      "id" : "43E4A7F7-23BD-4B48-9FF6-0C5DE03F97E9",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "lightgreen"
        ]
      ],
      "name" : "SPRING-3232\\nResponse Handler\\nDev testing",
      "pos" : {
        "x" : 505.478515625,
        "y" : 1
      }
    },
    {
      "id" : "5BD327E0-B8A3-4FBF-8A6E-D587374A5620",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ]
      ],
      "name" : "RM-13741 - Session laucnher\\nDev testing",
      "pos" : {
        "x" : 850.533203125,
        "y" : -131
      }
    },
    {
      "id" : "A0431EF1-7961-44EE-AC92-A89ADE68040F",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ]
      ],
      "name" : "SPRING-3232\\nResponse Handler\\nDev testing",
      "pos" : {
        "x" : 712.556640625,
        "y" : -213
      }
    },
    {
      "id" : "93D627CD-6A0B-40BD-A034-BE74FAF774AD",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ]
      ],
      "name" : "ITEST-15145\\nSLC Agent Support\\nDev testing",
      "pos" : {
        "x" : 1244.634765625,
        "y" : 3
      }
    },
    {
      "id" : "C881AA21-09F7-455C-996F-55DBC28A7FE5",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "ITEST-15262\\nSelenium IDE - Dev testing",
      "pos" : {
        "x" : 26.279296875,
        "y" : -440
      }
    },
    {
      "id" : "FD495D03-0CCE-4E7C-B725-CD938050707F",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "red"
        ]
      ],
      "name" : "SPRING Team",
      "pos" : {
        "x" : 731.2724609375,
        "y" : -312
      }
    },
    {
      "id" : "0D34409E-063F-4E9E-B9EA-E4A4CD43566C",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : -485,
        "y" : 200
      }
    },
    {
      "id" : "8BB54184-EBBE-4226-B048-A88A364DB2F5",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : -485,
        "y" : -500
      }
    },
    {
      "id" : "6CBD903A-ABD0-439C-85F7-A34B3BA9EFE8",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ],
        [
          "width",
          "200"
        ]
      ],
      "name" : "SPRING-3236\\nSLC ES testing",
      "pos" : {
        "x" : -430.720703125,
        "y" : -372
      }
    },
    {
      "id" : "52042F14-17A0-48E7-9AC9-4EEDDEF1C53B",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : -650,
        "y" : -400
      }
    },
    {
      "id" : "5791817E-9668-41DD-BFD8-7C685B5D066F",
      "kind" : "item",
      "properties" : [
        [
          "width",
          "15"
        ],
        [
          "height",
          "15"
        ]
      ],
      "name" : " ",
      "pos" : {
        "x" : 1600,
        "y" : -400
      }
    },
    {
      "id" : "E4C5BD27-AC0A-4A99-B662-BABE84F4722E",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ]
      ],
      "name" : "Complete tasks",
      "pos" : {
        "x" : -305,
        "y" : 185
      }
    },
    {
      "id" : "0FB1408B-2268-469C-A6A6-0E15CBA54E31",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "NDA-122\\nSNMP",
      "pos" : {
        "x" : -462.720703125,
        "y" : -447
      }
    },
    {
      "id" : "F84F16E2-AB99-47B1-9930-F2403C7C5AC2",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "ITEST-15229",
      "pos" : {
        "x" : -391.84375,
        "y" : -425
      }
    },
    {
      "id" : "21B9733F-13F5-42F3-B135-DCFB4EA4C932",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "ITEST-15292",
      "pos" : {
        "x" : -392.68359375,
        "y" : -454
      }
    },
    {
      "id" : "3F2B01A2-BBD9-4ABF-ACC8-A34116F3B16E",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "green"
        ]
      ],
      "name" : "ITEST-15303",
      "pos" : {
        "x" : -460.5234375,
        "y" : -479
      }
    },
    {
      "id" : "D354D9BF-B49D-4BF3-BDA6-A29D61C2982F",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "yellow"
        ]
      ],
      "name" : "ITEST-15196",
      "pos" : {
        "x" : -366.900390625,
        "y" : -482
      }
    },
    {
      "id" : "8D4852EF-AB75-429E-9FD8-F53A9A0A8F3B",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ]
      ],
      "name" : "ITEST-14679\\nSession Profile Editor Layout",
      "pos" : {
        "x" : 508.244140625,
        "y" : -447
      }
    },
    {
      "id" : "7533A609-6293-402C-8BFE-8ADEEBE9252A",
      "kind" : "item",
      "properties" : [
        [
          "color",
          "red"
        ]
      ],
      "name" : "Delay 1-2d",
      "pos" : {
        "x" : 414.984375,
        "y" : -238
      }
    },
    {
      "id" : "0919199B-D3B8-4355-92CB-284E28C56F5D",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "lightgreen"
        ]
      ],
      "name" : "SPRING-3227\\n ES Dev testing",
      "pos" : {
        "x" : 514.984375,
        "y" : -238
      }
    },
    {
      "id" : "BA1962B0-43B1-4DBF-9635-32931477EDCD",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "lightgreen"
        ]
      ],
      "name" : "RM-13965\\nPrompts form",
      "pos" : {
        "x" : 510.068359375,
        "y" : 103
      }
    },
    {
      "id" : "C6569372-21D1-4B02-8600-CA45F0FD355F",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "lightgreen"
        ]
      ],
      "name" : "RM-13096\\nTelnet Session",
      "pos" : {
        "x" : 604.908203125,
        "y" : 104
      }
    },
    {
      "id" : "9020C042-69E7-4D50-B3D3-1F7340348E44",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "lightgreen"
        ]
      ],
      "name" : "ITEST-15270\\nTestingTool enhancement",
      "pos" : {
        "x" : 38.6162109375,
        "y" : -374
      }
    },
    {
      "id" : "BE267933-8E9B-4AFB-967D-4F6B17AC6FDA",
      "kind" : "item",
      "properties" : [
        [
          "font-size",
          "12"
        ],
        [
          "color",
          "lightgreen"
        ]
      ],
      "name" : "ITEST-15270\\nTestingTool enhancement",
      "pos" : {
        "x" : 515.6123046875,
        "y" : -371
      }
    },
    {
      "id" : "5D1D5C09-1260-4233-9B43-F817F822D378",
      "kind" : "item",
      "name" : "ITEST-15294",
      "pos" : {
        "x" : 619,
        "y" : -238
      }
    }
  ],
  "edges" : [
    {
      "target" : "F45B9D19-960C-4A5D-8A08-9D336C7902CB",
      "source" : "4DD90AFC-DF85-4B5B-82C7-630424AA1259",
      "id" : "9AA31B0A-E49A-4CE1-A8EF-1B7300DA047F",
      "properties" : [
        [
          "display",
          "arrow"
        ],
        [
          "line-dash",
          "dashed"
        ]
      ],
      "kind" : "link",
      "name" : "",
      "pos" : {
        "x" : 0,
        "y" : 0
      }
    },
    {
      "target" : "3068C334-782C-4789-859F-90A5305A678D",
      "source" : "DE962CE9-E19C-4F72-9D58-D4C777C642BF",
      "id" : "8BEB1EAB-DDA4-4A71-92FA-979B9B028ADF",
      "properties" : [
        [
          "display",
          "arrow"
        ],
        [
          "line-dash",
          "dashed"
        ]
      ],
      "kind" : "link",
      "name" : "",
      "pos" : {
        "x" : 0,
        "y" : 0
      }
    },
    {
      "target" : "855142BB-DD3B-4280-9E17-1E2C35190255",
      "source" : "1451151F-F916-4334-92E2-3BCBFC14D8FE",
      "id" : "3CA49657-9737-4C61-97CA-FF46D8558C33",
      "properties" : [
        [
          "display",
          "arrow"
        ]
      ],
      "kind" : "link",
      "name" : "",
      "pos" : {
        "x" : 0,
        "y" : 0
      }
    },
    {
      "target" : "3D05C6C8-A98E-416C-80D3-5053355F5F25",
      "source" : "F819AC8E-493D-4CBE-951D-E7B866A99929",
      "id" : "FC8B88B8-9D66-4494-9633-B65195EA9AAB",
      "properties" : [
        [
          "display",
          "arrow"
        ],
        [
          "line-dash",
          "dashed"
        ]
      ],
      "kind" : "link",
      "name" : "",
      "pos" : {
        "x" : 0,
        "y" : 0
      }
    },
    {
      "target" : "89470140-0A5E-434B-97AF-9083C996BED1",
      "source" : "4D67EF7C-790B-4D8D-BB4E-F71BA6A5D5C5",
      "id" : "19B63E03-B8B2-46B5-ACD4-AFA163E63251",
      "properties" : [
        [
          "display",
          "arrow"
        ],
        [
          "line-dash",
          "dashed"
        ]
      ],
      "kind" : "link",
      "name" : "",
      "pos" : {
        "x" : 0,
        "y" : 0
      }
    },
    {
      "target" : "9EB4725C-580E-4E2E-A4C4-57F6206CCDB6",
      "source" : "521C7175-48C0-4045-954B-28F41B35E000",
      "id" : "4C5E85A1-F7C8-43C7-9F57-95F2EBFE02EA",
      "properties" : [
        [
          "display",
          "arrow"
        ],
        [
          "line-dash",
          "dashed"
        ]
      ],
      "kind" : "link",
      "name" : "",
      "pos" : {
        "x" : 0,
        "y" : 0
      }
    },
    {
      "target" : "C61332AE-8A72-4D8C-90DF-8D849AD284C2",
      "source" : "E1EA3E50-FAFF-417F-A325-0A41E0D12952",
      "id" : "8E2702C0-FE03-4B21-897D-5D152302F7CE",
      "properties" : [
        [
          "display",
          "arrow"
        ]
      ],
      "kind" : "link",
      "name" : "",
      "pos" : {
        "x" : 0,
        "y" : 0
      }
    },
    {
      "target" : "2A8A7ACF-535B-4401-9346-69B79DEDE39F",
      "source" : "2C098078-F722-485E-B85E-2C1F5F8C2772",
      "id" : "9BF0857E-178D-4082-96F6-3D0E75C916F8",
      "properties" : [
        [
          "display",
          "arrow"
        ]
      ],
      "kind" : "link",
      "name" : "",
      "pos" : {
        "x" : 0,
        "y" : 0
      }
    },
    {
      "target" : "5BCB6C6D-6849-49F6-9397-8F53356A8A8F",
      "source" : "5BCB6C6D-6849-49F6-9397-8F53356A8A8F",
      "id" : "0BBA34BD-FC05-4EDC-8F6E-6B3CEDDD6A8D",
      "properties" : [
        [
          "display",
          "arrow"
        ]
      ],
      "kind" : "link",
      "name" : "",
      "pos" : {
        "x" : 0,
        "y" : 0
      }
    },
    {
      "target" : "3BD28E50-7D53-44B1-AF82-162A665D1083",
      "source" : "18513366-9E6F-40FB-BC68-6F427596AA7A",
      "id" : "60FFD501-E5AB-473F-9B00-FEB29567ECE5",
      "properties" : [
        [
          "display",
          "arrow"
        ],
        [
          "line-dash",
          "dashed"
        ]
      ],
      "kind" : "link",
      "name" : "",
      "pos" : {
        "x" : 0,
        "y" : 0
      }
    },
    {
      "target" : "29E4ADC7-D000-4697-B9FF-B34297ABAA40",
      "source" : "CE88F4BC-AF65-4DF7-8898-FEBF55C0F35F",
      "id" : "B6279949-48FB-4354-AC72-B7F22870CBE5",
      "properties" : [
        [
          "display",
          "arrow"
        ],
        [
          "line-dash",
          "dashed"
        ]
      ],
      "kind" : "link",
      "name" : "",
      "pos" : {
        "x" : 0,
        "y" : 0
      }
    },
    {
      "target" : "CD959844-436B-4188-9C3B-508E156E8578",
      "source" : "7DE4BE2A-7E89-43C9-9998-D0DE38AD2C89",
      "id" : "E8837428-BE1C-48FA-8892-FA5137EB24B3",
      "properties" : [
        [
          "display",
          "arrow"
        ],
        [
          "line-dash",
          "dashed"
        ]
      ],
      "kind" : "link",
      "name" : "",
      "pos" : {
        "x" : 0,
        "y" : 0
      }
    },
    {
      "target" : "9212173F-77CF-42B1-B2F7-EBEC1E45346D",
      "source" : "616FB269-7D4B-4320-8E87-16A7697CE36B",
      "id" : "F638BE5F-5A43-469F-A96C-C8CA9B31FE7D",
      "properties" : [
        [
          "display",
          "arrow"
        ],
        [
          "line-dash",
          "dashed"
        ]
      ],
      "kind" : "link",
      "name" : "",
      "pos" : {
        "x" : 0,
        "y" : 0
      }
    },
    {
      "target" : "FD495D03-0CCE-4E7C-B725-CD938050707F",
      "source" : "522BF915-B55E-41D9-88D7-6B242E133BC3",
      "id" : "46D97B95-931E-4CAE-AE69-43B7616BF5D4",
      "properties" : [
        [
          "display",
          "arrow"
        ]
      ],
      "kind" : "link",
      "name" : "blocked",
      "pos" : {
        "x" : 0,
        "y" : 0
      }
    },
    {
      "target" : "8BB54184-EBBE-4226-B048-A88A364DB2F5",
      "source" : "0D34409E-063F-4E9E-B9EA-E4A4CD43566C",
      "id" : "4084722C-04D7-4EF2-AE6B-B0DC44DA85F7",
      "properties" : [
        [
          "display",
          "arrow"
        ],
        [
          "line-dash",
          "dashed"
        ]
      ],
      "kind" : "link",
      "name" : "",
      "pos" : {
        "x" : 0,
        "y" : 0
      }
    },
    {
      "target" : "5791817E-9668-41DD-BFD8-7C685B5D066F",
      "source" : "52042F14-17A0-48E7-9AC9-4EEDDEF1C53B",
      "id" : "E618CA19-C409-4421-A214-8458FF39524F",
      "properties" : [
        [
          "display",
          "arrow"
        ],
        [
          "line-dash",
          "dashed"
        ]
      ],
      "kind" : "link",
      "name" : "",
      "pos" : {
        "x" : 0,
        "y" : 0
      }
    },
    {
      "id" : "86E908F5-E445-4D5A-A8A0-C0502D83B777",
      "kind" : "link",
      "source" : "D0B3A748-0169-411E-9834-97F1B6DDAD34",
      "name" : "",
      "pos" : {
        "x" : 0,
        "y" : 0
      },
      "target" : "7533A609-6293-402C-8BFE-8ADEEBE9252A"
    },
    {
      "target" : "7533A609-6293-402C-8BFE-8ADEEBE9252A",
      "source" : "0919199B-D3B8-4355-92CB-284E28C56F5D",
      "id" : "E04DC190-03D3-4240-A793-A9823C390BB0",
      "properties" : [
        [
          "display",
          "arrow"
        ]
      ],
      "kind" : "link",
      "name" : "",
      "pos" : {
        "x" : 0,
        "y" : 0
      }
    }
  ]
}
"""

sync_data_obj = json.loads(sync_data)

existing_issues = {}

items = sync_data_obj['items']

for item in items:
	names = item['name'].replace('\\\\','\\').split('\n')
	name = names[0]
	if name.find(' -') != -1:
		name = name[0:name.find(' -')]
	name = name.strip()
	if name.startswith("ITEST-") or name.startswith("SPRING-") or name.startswith("RM-"):
		# print ("item:", name )
		nodes = existing_issues.get(name)
		if nodes == None:
			nodes = []
		nodes.append(item)
		existing_issues[name] =  nodes

unresolved_major_stories = """project in ("Spirent iTest", "Integration Engineering", "Network Devops Agent", "Resource Manager", Springboard) and assignee in ('sanokhin',  'dsheblykin',  'asobolev', 'rsychev',  'eykhman') AND issuetype = Story AND fixVersion in (7.2.0) AND priority >= 1-High"""

# user = input('User: ')
user = 'asobolev'
# password = getpass('Password: ')
password = 'jG3FmobV'

save_requests = True

def get_jira_data():
	jira_url='https://jira.spirenteng.com'
	jira = JIRA(jira_url, basic_auth=(user, password))

	stories = jira.search_issues(unresolved_major_stories)
	user_stories = {}
	members = ['sanokhin',  'dsheblykin',  'asobolev', 'rsychev',  'eykhman', 'wdai']

	json_data = {}

	for story in stories:
		assignee = story.fields.assignee.name
		if assignee in members:
			issue_data = existing_issues.get(story.key)
			if issue_data == None:
				print('Story Not Found:', story.key, ' name:', story.fields.summary )
			else:
				print('Story Found:', story.key, ' name:', story.fields.summary )

			stories = user_stories.get(assignee)
			if stories == None:
				stories = []
			user_stories[assignee] = stories

			subtasks = jira.search_issues('parent = {0}'.format(story.key))
			for subtask in subtasks:
				if subtask.fields.assignee.name in members:
					summary = subtask.fields.summary
					pos1 = summary.find('--')
					if pos1 != -1:
						summary = summary[pos1+2:]
					pos2 = summary.find('Implementation:')
					if pos2 != -1:
						summary = summary[pos2:]

					sub_issue_data = existing_issues.get(subtask.key)
					summary = summary.strip()
					if summary in ['Test Plan Review', 'Code Review', 'Code review', 'Post-Demo Improvements']:
						continue
					if sub_issue_data == None:
						print('Subtask Not Found:', subtask.key, ' name:', story.fields.summary + " - " +  summary )
						## Add subtask item
						item =  {
							"id" : str(uuid.uuid1()),
							"kind" : "item",
							"properties" : [
								["font-size","12"],
								["color", "white"]
							],
							"name" : subtask.key + '\\n' + story.fields.summary + " - " +  summary ,
							"pos" : { "x" : 0, "y" : 0}
						}
						items.append(item)

	print('users', user_stories.keys())

get_jira_data()

print("-----RESULT----")
print (json.dumps(sync_data_obj))
