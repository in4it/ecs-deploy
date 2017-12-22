import { Component, OnInit, ViewChild } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { Observable } from 'rxjs/Observable';

import { ServiceDetail, ServiceDetailService }  from './service-detail.service';
import { InspectChildComponent }  from './inspect.component';
import { DeployChildComponent }  from './deploy.component';

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

  selectedVersion: any

  tab = "service"

  @ViewChild(InspectChildComponent) inspectChild;
  @ViewChild(DeployChildComponent) deployChild;

  constructor(
    private route: ActivatedRoute,
    private router: Router,
    private sds: ServiceDetailService
  ) {}

  ngOnInit(): void {
    this.route.data
      .subscribe((data: { sd: ServiceDetail }) => {
        this.formatServiceData(data.sd.service)
     });
  }

  onClickVersions() {
    this.versions = [];
    this.tab = "versions"
    this.loading = true
    this.sds.getVersions().subscribe(data => {
      this.loading = false
      let versionMap = {}
      data['versions'].forEach((version, index) => {
        let lastDeployMoment = moment(version.lastDeploy);
        data['versions'][index]['lastDeployMoment'] = lastDeployMoment.fromNow()
        versionMap[version.lastDeploy] = version
      })
      this.versions = data['versions'];
      this.deployChild.setVersionMap(versionMap)
    });
  }
  onClickService() {
    this.tab = "service"
  }
  onClickEvents() {
    this.tab = "events"
  }
  refresh() {
    this.loading = true
    this.sds.getService(this.service.serviceName).subscribe(data => {
      this.loading = false
      this.formatServiceData(data["service"])
    });
  }

  formatServiceData(service): void {
    service["deploymentMap"] = {}
    // format deployments
    service["deployments"].forEach((deployment, index) => {
      // make a map per status of deployments
      let lastDeploy = moment(deployment.createdAt).format('YYYY-MM-DD hh:mm:ss Z');
      deployment.lastDeploy = lastDeploy;
      service["deploymentMap"][deployment["status"]] = deployment
    })
    // format events
    service["events"].forEach((serviceEvent, index) => {
      let eventFormatted = moment(serviceEvent.createdAt).format('YYYY-MM-DD hh:mm:ss Z');
      service["events"][index]["createdAtFormatted"] = eventFormatted
    })
    // format tasks
    service["taskStatus"] = {}
    service["taskTotal"] = 0
    service["containerStatus"] = {}
    service["containerTotal"] = 0
    service["tasks"].forEach((task, index) => {
      service["taskTotal"]++
      if(service["taskStatus"][task["lastStatus"]]) {
        service["taskStatus"][task["lastStatus"]]++
      } else {
        service["taskStatus"][task["lastStatus"]] = 1
      }
      task["containers"].forEach((container, index) => {
        service["containerTotal"]++
        if(service["containerStatus"][container["lastStatus"]]) {
          service["containerStatus"][container["lastStatus"]]++
        } else {
          service["containerStatus"][container["lastStatus"]] = 1
        }
      })
    })
    this.service = service
  }

  deploying(loading) {
    if(loading) {
      this.loading = loading
    }
  }
  deployed() {
    this.loading = true
    this.tab = "service"
    this.sds.getService(this.service.serviceName).subscribe(data => {
      this.loading = false
      this.formatServiceData(data["service"])
    });
  }
  

}
