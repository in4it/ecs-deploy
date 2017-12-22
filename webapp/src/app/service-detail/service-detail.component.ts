import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { Observable } from 'rxjs/Observable';

import { ServiceDetail, ServiceDetailService }  from './service-detail.service';

import * as moment from 'moment';

@Component({
  selector: 'app-service-detail',
  templateUrl: './service-detail.component.html',
  styleUrls: ['./service-detail.component.css']
})
export class ServiceDetailComponent implements OnInit {

  service: any = {};
  versions: any = {};
  loading: boolean = false;

  tab = "service"

  constructor(
    private route: ActivatedRoute,
    private router: Router,
    private sds: ServiceDetailService
  ) {}

  ngOnInit(): void {
    this.route.data
      .subscribe((data: { sd: ServiceDetail }) => {
        this.service = data.sd.service
     });
  }

  onClickVersions() {
    this.versions = [];
    this.tab = "versions"
    this.loading = true
    this.sds.getVersions().subscribe(data => {
      this.loading = false
      data['versions'].forEach((version, index) => {
        let lastDeployMoment = moment(version.lastDeploy);
        data['versions'][index]['lastDeployMoment'] = lastDeployMoment.fromNow()
      })
      this.versions = data['versions'];
    });
  }
  onClickService() {
    this.tab = "service"
  }

}
