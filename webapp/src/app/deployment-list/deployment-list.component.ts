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
  filterList: string[] = [];

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
        this.filterList = []
     });
  }

  filter(event: any, serviceName: string) {
    if(event.target.checked) {
      this.filterList.push(serviceName)
    } else {
      this.filterList = this.filterList.filter(a => a !== serviceName)
    }

    this.ds.getDeploymentList(serviceName).subscribe((data: DeploymentList ) => {
      if(data.deployments.length != 0) {
        this.deployments = data.deployments.filter((deployment) => {
          if (this.filterList.length === 0) { return true }
          for (var i = 0; i < this.filterList.length; i++) {
            if(this.filterList[i] == deployment["ServiceName"]) {
              return true
            }
          }
          return false
        })
      }
    });
  }
}
