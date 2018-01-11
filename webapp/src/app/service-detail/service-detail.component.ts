import { Component, OnInit, ViewChild } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { Observable } from 'rxjs/Observable';

import { ServiceDetail, ServiceDetailService }  from './service-detail.service';
import { InspectChildComponent }  from './inspect.component';
import { DeployChildComponent }  from './deploy.component';
import { ConfirmChildComponent }  from './confirm.component';

import { AlertService } from '../services/index';

import * as moment from 'moment';

@Component({
  selector: 'app-service-detail',
  templateUrl: './service-detail.component.html',
  styleUrls: ['./service-detail.component.css']
})
export class ServiceDetailComponent implements OnInit {

  service: any = {};
  versions: any = {};
  parameters: any = {};
  loading: boolean = false;
  saving: boolean = false;

  selectedParameter: string = "";
  newParameter: boolean = false;
  newParameterInput: any = {};
  parameterInput: any = {};

  selectedVersion: any;

  editManualScaling: boolean = false;
  scalingInput: any = {};

  tab = "service"

  @ViewChild(InspectChildComponent) inspectChild;
  @ViewChild(DeployChildComponent) deployChild;
  @ViewChild(ConfirmChildComponent) confirmChild;

  constructor(
    private route: ActivatedRoute,
    private router: Router,
    private sds: ServiceDetailService,
    private alertService: AlertService
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
  onClickScaling() {
    this.tab = "scaling"
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
  deployed(deployResult) {
    this.loading = true
    this.tab = "service"
    this.sds.getService(this.service.serviceName).subscribe(data => {
      this.loading = false
      this.formatServiceData(data["service"])
    });
  }

  /*
   *
   *  Parameters
   *
   */
  onClickParameters() {
    this.parameters = [];
    this.tab = "parameters"
    this.loading = true
    this.sds.listParameters().subscribe(data => {
      this.loading = false
      this.parameters["keys"] = []
      this.parameters["map"] = data['parameters'];
      for (let key in this.parameters["map"]) {
        this.parameters["keys"].push(key)
      }
    });
  }
  
  showNewParameter() {
    this.newParameter = true
  }
  saveNewParameter() {
    if("name" in this.newParameterInput && "value" in this.newParameterInput) {
      this.saving = true
      this.sds.putParameter(this.newParameterInput).subscribe(data => {
        this.saving = false
        this.newParameterInput = {}
        this.newParameter = false
        this.onClickParameters()
      });
    }
  }
  editParameter(parameter) {
    this.selectedParameter = parameter
    this.parameterInput["value"] = this.parameters["map"][parameter]["value"]
    if(this.parameters["map"][parameter]["type"] == "SecureString") {
      this.parameterInput["encrypted"] = true
    } else {
      this.parameterInput["encrypted"] = false
    }
    this.parameterInput["name"] = parameter
  }
  saveParameter(parameter): void {
    if("value" in this.parameterInput) {
      this.saving = true
      this.sds.putParameter(this.parameterInput).subscribe(data => {
        if(this.parameters["map"][parameter]["type"] == "SecureString") {
          this.parameters["map"][parameter]["value"] = "***"
        } else {
          this.parameters["map"][parameter]["value"] = this.parameterInput["value"]
        }
        this.saving = false
        this.selectedParameter = ""
        this.parameterInput = {}
      });
    }
  }
  
  deletingParameter(loading) {
    if(loading) {
      this.loading = loading
    }
  }
  deletedParameter(selectedParameter) {
    this.loading = true
    delete this.parameters["map"][selectedParameter]
    this.parameters["keys"] = []
    for (let key in this.parameters["map"]) {
      this.parameters["keys"].push(key)
    }
    this.loading = false
  }
  
  editDesiredCount() {
    this.scalingInput.desiredCount = this.service.desiredCount
    this.editManualScaling = true
  }
  saveDesiredCount(): void {
    if("desiredCount" in this.scalingInput) {
      this.saving = true
      this.sds.setDesiredCount(this.scalingInput).subscribe(data => {
        if(data["message"] != "OK") {
          this.alertService.error(data["error"]);
        }
        this.service["desiredCount"] = this.scalingInput["desiredCount"]
        this.saving = false
        this.editManualScaling = false
        this.scalingInput = {}
      });
    }
  }

}
