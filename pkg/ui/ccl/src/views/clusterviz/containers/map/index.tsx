// Copyright 2017 The Cockroach Authors.
//
// Licensed under the Cockroach Community Licence (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://github.com/cockroachdb/cockroach/blob/master/licenses/CCL.txt

import React from "react";
import {InjectedRouter, RouterState} from "react-router";

import { Breadcrumbs } from "src/views/clusterviz/containers/map/breadcrumbs";
import NodeCanvasContainer from "src/views/clusterviz/containers/map/nodeCanvasContainer";
import TimeScaleDropdown from "src/views/cluster/containers/timescale";
import Dropdown, { DropdownOption } from "src/views/shared/components/dropdown";
import { parseLocalityRoute } from "src/util/localities";

import "./tweaks.styl";

export default class ClusterVisualization extends React.Component<RouterState & { router: InjectedRouter }> {
  handleMapTableToggle = (opt: DropdownOption) => {
    this.props.router.push(`/overview/${opt.value}`);
  }

  render() {
    const tiers = parseLocalityRoute(this.props.params.splat);
    const options: DropdownOption[] = [
      { value: "map", label: "Node Map" },
      { value: "list", label: "Node List" },
    ];

    // TODO(vilterp): dedup with NodeList
    return (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          flexDirection: "column",
          overflow: "hidden",
        }}
        className="clusterviz"
      >
        <div style={{
          flex: "none",
          backgroundColor: "white",
          boxShadow: "0 0 4px 0 rgba(0, 0, 0, 0.2)",
          zIndex: 5,
          padding: "4px 12px",
        }}>
          <div style={{ float: "left" }}>
            <Dropdown
              title="View"
              selected="map"
              options={options}
              onChange={this.handleMapTableToggle}
            />
          </div>
          <div style={{ float: "right" }}><TimeScaleDropdown /></div>
          <div style={{ textAlign: "center", paddingTop: 4 }}><Breadcrumbs tiers={tiers} /></div>
        </div>
        <NodeCanvasContainer tiers={tiers} />
      </div>
    );
  }
}
