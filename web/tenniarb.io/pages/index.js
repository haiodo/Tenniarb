import Head from 'next/head';
import Link from 'next/link';
import React from 'react';

import Router from 'next/router';

import "../static/css/menu.css"
import "../static/css/side-menu.css"

var Pages = {
    Features: 'features',
    Documentation: 'documenation',
    About: 'about',
    Downloads: 'downloads',
}

var DocumentationPages = {
    General: 'general',
    Layout: 'layout',
    Colors: 'colors',
    Styles: 'styles',
    Executions: 'executions',
    Export: 'export'
}

class Features extends React.Component {
    render() {
        return <div class="main">
            <h2>Features</h2>
            <p>This sidebar is of full height (100%) and always shown.</p>
            <p>Scroll down the page to see the result.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
        </div>
    }
}
class About extends React.Component {
    render() {
        return <div class="main">
            <h2>Features</h2>
            <p>This sidebar is of full height (100%) and always shown.</p>
            <p>Scroll down the page to see the result.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
        </div>
    }
}

class Downloads extends React.Component {
    render() {
        return <div class="main">
            <h2>Features</h2>
            <p>This sidebar is of full height (100%) and always shown.</p>
            <p>Scroll down the page to see the result.</p>
            <p>Some text to enable scrolling.. Lorem ipsum dolor sit amet, illum definitiones no quo, maluisset concludaturque et eum, altera fabulas ut quo. Atqui causae gloriatur ius te, id agam omnis evertitur eum. Affert laboramus repudiandae nec et. Inciderint efficiantur his ad. Eum no molestiae voluptatibus.</p>
        </div>
    }
}

function DocsGenralInfo() {
    return <div class="main">
        <h2>General Information</h2>
        <p>
            <h4>Add of new item to diagram:</h4>
            <table>
                <tr>
                    <td>
                        <ul>
                            <li>1. Press 'Tab' to add new item to diagram</li>
                            <li>2. Select item and press 'Tab' to add linked item to diagram.</li>
                        </ul>
                    </td>
                    <td>
                        <img src="/static/images/docs/basic_steps_1.png" width="300px" />
                    </td>
                </tr>
            </table>

        </p>

    </div>
}

function DocsOther() {
    return <div class="main">
        <h2>Documention not defined</h2>
        <p>No documentation yet</p>
    </div>
}

function SideMenuItem(props) {
    var hrefValue = "?page=" + Pages.Documentation + "&" + "docPageId=" + props.pageId
    return <Link href={hrefValue}>
        <a className={props.curPageId == props.pageId ? "active_item" : ""} href={hrefValue}>{props.title}</a>
    </Link>
}


class Documentation extends React.Component {
    render() {
        var page = "";
        var pageId = "";
        if (Router.router != null) {
            pageId = Router.router.query["docPageId"];
        }
        if (pageId == null || pageId.length == 0) {
            pageId = DocumentationPages.General;
        }
        switch (pageId) {
            case DocumentationPages.General:
                page = <DocsGenralInfo />;
                break;
            case DocumentationPages.Layout:
                page = <DocsOther />;
                break;
            default:
                page = <DocsOther />;
                break;
        }

        return <div>
            <div class="sidenav">
                <SideMenuItem pageId={DocumentationPages.General} curPageId={pageId} title="General" />
                <SideMenuItem pageId={DocumentationPages.Layout} curPageId={pageId} title="Layout" />
                <SideMenuItem pageId={DocumentationPages.Colors} curPageId={pageId} title="Colors" />
                <SideMenuItem pageId={DocumentationPages.Styles} curPageId={pageId} title="Styles" />
                <SideMenuItem pageId={DocumentationPages.Executions} curPageId={pageId} title="Executions" />
                <SideMenuItem pageId={DocumentationPages.Export} curPageId={pageId} title="Export" />
            </div>
            <div class="main_side">
                {page}
            </div>
        </div>
    }
}

function MenuItem(pageId, curPageId, title) {
    var hrefId = "?page=" + pageId
    return <Link href={hrefId}>
        <li><a href={hrefId} className={curPageId === pageId ? "active_menu" : ""} >{title}</a></li>
    </Link>
}

class IndexPage extends React.Component {
    render() {
        var page = "";
        var pageId = "";
        if (Router.router != null) {
            pageId = Router.router.query["page"];
        }
        if (pageId == null || pageId.length == 0) {
            pageId = Pages.Features;
        }
        switch (pageId) {
            case Pages.Documentation:
                page = <Documentation />;
                break;
            case Pages.Features:
                page = <Features />;
                break;
            case Pages.About:
                page = <About />;
                break;
            case Pages.Downloads:
                page = <Downloads />;
                break;
            default:
                page = <Features />;
                break;
        }
        return <div>
            <Head>
                <title>Tenniarb.io documentaion</title>
                <meta name="viewport" content="initial-scale=1.0, width=device-width" />
            </Head>
            <header>
                <a className="logo" href="/">
                    <img src="/static/images/Icon.png" width="16px" height="16px" />
                    {" "} Tenniarb
                </a>

                {/* <input id="nav" type="checkbox" />
                <label for="nav"></label> */}

                <nav>
                    <ul>
                        {MenuItem(Pages.Features, pageId, "Features")}
                        {MenuItem(Pages.Documentation, pageId, "Documentation")}
                        {MenuItem(Pages.Downloads, pageId, "Downloads")}
                        {MenuItem(Pages.About, pageId, "About")}
                    </ul>
                </nav>
            </header>
            <div>
                {page}
            </div>
        </div >;
    }
}

export default IndexPage;
