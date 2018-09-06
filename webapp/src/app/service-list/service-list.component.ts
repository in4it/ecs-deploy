import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { Observable } from 'rxjs';

import { ServiceList, ServiceListService }  from './service-list.service';

import * as moment from 'moment';

@Component({
  selector: 'app-service-list',
  templateUrl: './service-list.component.html',
  styleUrls: ['./service-list.component.css']
})
export class ServiceListComponent implements OnInit {

  services: string[] = [];

  constructor(
    private route: ActivatedRoute,
    private router: Router,
    private sls: ServiceListService
  ) {}

  ngOnInit(): void {
    this.route.data
      .subscribe((data: { sl: ServiceList }) => {
        let services = data.sl.services
        services.forEach((service, index) => {
          services[index]["deploymentMap"] = {}
          service["deployments"].forEach((deployment, index2) => {
            // make a map per status of deployments
            let lastDeploy = moment(deployment.updatedAt);
            deployment.lastDeploy = lastDeploy.fromNow();
            services[index]["deploymentMap"][deployment["status"]] = deployment
          })
        })
        this.services = services;
     });
  }

}
