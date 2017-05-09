import * as React from "react";
import * as ReactDOM from "react-dom";

import { Hello } from "./components/Hello";

import styled from 'styled-components';

const { Window, TitleBar, Text, Toolbar } = require('react-desktop/macOs');

const MainLayout = styled.div`
    display: grid;
    height: 100%;
    margin: 0 0 0 0;
    padding: 0 0 0 0;
    grid-template-columns: 220px auto;
    grid-template-rows: 40px auto 180px 20px;
    grid-template-areas: 
        'title title' 
        'nav main' 
        'nav props' 
        'footer footer'
`

const TitlePane = styled.div`
    grid-area: title;
`

const NavPanel = styled.div`
    background: #FFFFFF;
    border: 1px solid #93A1A1;
    grid-area: nav;
`

const ScenePanel = styled.div`
    grid-area: main;
    background: #FDF6E3;
    border: 1px solid #93A1A1;
    position: relative;
    left: 0px;
    top: 0px;
`
const PropsPanel = styled.div`
    background: #FFFFFF;
    border: 1px solid #93A1A1;
    grid-area: props;
`

const StatusPanel = styled.div`
    background: #FFFFFF;
    border: 1px solid #93A1A1;
    grid-area: footer;
`

const SceneItem = styled.div`
    background: #FFFFFF;
    padding: -1px -1px;
    border: ${props => props.theme.selected ? '2px solid #073642' : ''};
    box-shadow: 2px 2px 4px 0 rgba(0, 0, 0, 0.50);
    border-radius: 9px;
    position: absolute;
    height: 50px;
    line-height: 48px;
    width: 131px;
    vertical-align: middle;
    user-select: none;
    cursor: pointer;
`

const SceneText = styled.div`
    font-family: ArialMT;
    font-size: 24px;
    color: #268BD2;
    text-align: center;
`


export class MainPlain extends React.Component<any, any> {
    render() {
        return (
            <MainLayout>
                <TitlePane height={45}>
                    <TitleBar controls inset>
                        <Toolbar height="43" horizontalAlignment="center" />
                    </TitleBar>
                </TitlePane>
                <NavPanel>
                    Nav
                </NavPanel>
                <PropsPanel>
                    props
                </PropsPanel>
                <StatusPanel>
                    Status
                </StatusPanel>
                <ScenePanel>
                    <SceneItem>
                        <SceneText>Platform</SceneText>
                    </SceneItem>
                </ScenePanel>


            </MainLayout>
        );
    }
}

ReactDOM.render(
    <MainPlain />,
    document.getElementById("example")
);

