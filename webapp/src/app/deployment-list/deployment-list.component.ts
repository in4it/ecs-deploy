import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { Observable } from 'rxjs';

import { DeploymentList, DeploymentListService }  from './deployment-list.service';


@Component({
  selector: 'app-deployment-list',
  templateUrl: './deployment-list.component.html',
  styleUrls: ['./deployment-list.component.css']
})

export class DeploymentListComponent implements OnInit {
  services: string[] = [];
  deployments: string[] = [];
  filterActive: boolean = false

  constructor(
    private route: ActivatedRoute,
    private router: Router,
    private ds: DeploymentListService
  ) {}

  ngOnInit(): void {
    this.route.data
      .subscribe((data: { dl: DeploymentList }) => {
        this.deployments = data.dl.deployments;
        this.services = data.dl.services;
     });
  }

  filter(event: any, serviceName: string) {
    console.log("Filter: " + serviceName + ": " + event.target.checked)
    if(event.target.checked) {
      this.ds.getDeploymentList(serviceName).subscribe((data: DeploymentList ) => {
        if(data.deployments.length != 0) {
          if(this.filterActive) {
            // merge if filter is active
            this.deployments = [ ...this.deployments, ...data.deployments];
          } else {
            this.deployments = data.deployments
          }
          this.deployments.sort(function(a,b) {return (a["Time"] > b["Time"]) ? -1 : ((b["Time"] > a["Time"]) ? 1 : 0);} ); 
          this.filterActive = true
        }
      });
    } else {
      var newDeployments = [];
      for (let deployment of this.deployments) {
        if(deployment["ServiceName"] != serviceName) {
          newDeployments.push(deployment)
        }
      }
      if(newDeployments.length != this.deployments.length) {
        this.deployments = newDeployments
      }
      if(newDeployments.length == 0) {
        this.filterActive = false
        this.ds.getDeploymentList("").subscribe((data: DeploymentList ) => {
          if(data.deployments.length != 0) {
            this.deployments = data.deployments
          }
        });
      }
    }
  }

}
