import React from 'react';
import Level from './Level';

const UpdateLevel = (props) => {
    let levels = [];
    for(let i=0; i < props.levels.length; i++) {
        let level = props.levels[i];
        let currentLevel = level.level.split('-')[level.level.split('-').length - 1];
        levels.push(<Level level={level} currentLevel={currentLevel} aggregator={props.aggregator} key={i} />)
    }


    return (
        <>
            {levels}
        </>
    );
}

export default UpdateLevel;