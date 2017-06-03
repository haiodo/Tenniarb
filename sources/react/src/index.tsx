import * as React from "react";
import * as ReactDOM from "react-dom";

// import { Hello } from "./components/Hello";

import styled from 'styled-components';

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
    left: ${props => props.theme.x + 'px'};
    top: ${props => props.theme.y + 'px'};
`

const SceneText = styled.div`
    font-family: ArialMT;
    font-size: 24px;
    color: #268BD2;
    text-align: center;
`

interface ISceneItemPane {
    x?: number;
    y?: number;
    text: string;
}
interface ISceneItemState {
    inMove?: boolean;
    x: number;
    y: number;
}
class SceneItemPane extends React.Component<ISceneItemPane, ISceneItemState> {
    state: ISceneItemState = { inMove: false, x: 0, y: 0 }
    constructor(props: ISceneItemPane) {
        super(props);
        this.state.x = props.x;
        this.state.y = props.y;
    }
    render() {
        return (
            <SceneItem ref={"item"} theme={{ x: this.state.x, y: this.state.y }} onMouseDown={
                () => { this.setState({ inMove: true }) }
            } onMouseUp={
                () => { this.setState({ inMove: false }) }
            } onMouseMove={(e) => {
                let item = ReactDOM.findDOMNode(this.refs["item"]) as HTMLElement;
                let rect = (item.parentElement as HTMLElement).getBoundingClientRect()
                if (this.state.inMove) this.setState({ x: e.pageX - rect.left - 75, y: e.pageY - rect.top - 25 });
            }} onMouseOut={() => { this.setState({ inMove: false }) }}>
                <SceneText>{this.props.text}</SceneText>
            </SceneItem>
        )
    }
}


export class MainPlain extends React.Component<any, any> {
    render() {
        return (
            <MainLayout>
                {/*<TitlePane height={45}>
                    <TitleBar controls inset>
                        <Toolbar height="43" horizontalAlignment="center" />
                    </TitleBar>
                </TitlePane>*/}
                <NavPanel>
                    Nav
                </NavPanel>
                <PropsPanel>
                    props
                </PropsPanel>
                <StatusPanel>
                    Status
                </StatusPanel>
                <ScenePanel ref={"diagram"}>
                    <SceneItemPane x={20} y={30} text={"Platform"} />

                    <SceneItemPane x={20} y={120} text={"Device"} />
                </ScenePanel>


            </MainLayout >
        );
    }
}

ReactDOM.render(
    <MainPlain />,
    document.getElementById("example")
);

