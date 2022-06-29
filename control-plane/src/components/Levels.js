import React from "react"
import Level from "./AddLevel"

class Levels extends React.Component {
    constructor(props) {
        super(props)
        this.state = {
            levels: [
                <Level currentLevel={1} aggregator={this.props.aggregator} key={1} />
            ]
        }
    }

    componentDidMount() {
        if(this.props.aggregator == 1) {
            window.levelOne = this;
        } else {
            window.levelTwo = this;
        }
      }

    addLevel = (currentLevel, aggregator) => {
        this.setState((state, props) => ({
            levels: [...state.levels, <Level currentLevel={currentLevel} aggregator={aggregator} key={currentLevel} /> ]
        }))
    }

    render() {
        return(
            <>
                {this.state.levels}
            </>
        );
    }
}

export default Levels;