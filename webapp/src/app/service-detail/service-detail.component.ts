import { Component, OnInit, ViewChild } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { Observable } from 'rxjs';

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
  loadingLogs: boolean = false;

  selectedParameter: string = "";
  newParameter: boolean = false;
  newParameterInput: any = {};
  parameterInput: any = {};

  selectedVersion: any;

  editManualScaling: boolean = false;
  scalingInput: any = {};
  scalingInputPolicy: any = {};

  runTaskInput: any = {};
  runTaskConfig: any = { "maxExecutionTime": 900};

  logsInput: any = {};

  tab = "service"

  @ViewChild(InspectChildComponent, { static: true }) inspectChild;
  @ViewChild(DeployChildComponent, { static: true }) deployChild;
  @ViewChild(ConfirmChildComponent, { static: true }) confirmChild;

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
    this.loading = true
    this.sds.getAutoscaling().subscribe(data => {
      this.loading = false
      if(data["autoscaling"]["minimumCount"] != 0 && data["autoscaling"]["maximumCount"] != 0) {
        this.scalingInput.minimumCount = data["autoscaling"]["minimumCount"]
        this.scalingInput.maximumCount = data["autoscaling"]["maximumCount"]
        if(data["autoscaling"]["policies"] && data["autoscaling"]["policies"].constructor === Array) {
          this.scalingInput.policyCount = data["autoscaling"]["policies"].length
        } else {
          this.scalingInput.policyCount = 0
        }
        this.scalingInput.desiredCount = this.service.desiredCount
        this.scalingInput.autoscaling = true
        if(data["autoscaling"]["policies"]) {
          this.scalingInput.policies = data["autoscaling"]["policies"]
          this.scalingInput.policies.forEach((policy, index) => {
            if(policy["scalingAdjustment"] > 0) {
              this.scalingInput.policies[index]["scalingAdjustment"] = "Up (+"+policy["scalingAdjustment"]+")"
            } else {
              this.scalingInput.policies[index]["scalingAdjustment"] = "Down ("+policy["scalingAdjustment"]+")"
            }
            switch(policy["comparisonOperator"]) {
              case "GreaterThanOrEqualToThreshold": {
                this.scalingInput.policies[index]["comparisonOperator"] = ">="
                break;
              }
              case "LessThanOrEqualToThreshold": {
                this.scalingInput.policies[index]["comparisonOperator"] = "<="
                break;
              }
              case "GreaterThanThreshold": {
                this.scalingInput.policies[index]["comparisonOperator"] = ">"
                break;
              }
              case "LessThanThreshold": {
                this.scalingInput.policies[index]["comparisonOperator"] = "<"
                break;
              }
            }
          })
        }
      }
    })
  }
  onClickRunTask() {
    this.tab = "runTask"
    this.loading = true
    this.sds.getTaskDefinition().subscribe(data => {
      this.loading = false
      if("taskDefinition" in data) {
        this.service["taskDefinition"] = data["taskDefinition"]
        this.service["taskDefinition"]["containerDefinitions"].forEach((container, index) => {
          this.runTaskInput[container["name"]] = {}
          if(container["name"] == this.service.serviceName) {
            this.runTaskInput[container["name"]]["enabled"] = true
          } else {
            this.runTaskInput[container["name"]]["enabled"] = false
          }
        })
      }
    });
  }
  onClickLogs(loading) {
    this.tab = "logs"
    this.loading = loading
    // default timeranges
    this.logsInput["timerange"] = [
      { id: "last-24h", name: "Last 24 hours" },
      { id: "last-7d", name: "Last 7 days" },
      { id: "last-14d", name: "Last 14 days" },
      { id: "last-30d", name: "Last 30 days" },
      { id: "last-1y", name: "Last 1 year" },
    ]
    if(!("selectedTimerange" in this.logsInput)) {
      this.logsInput.selectedTimerange = this.logsInput["timerange"][0]
    }
    this.sds.getTaskDefinition().subscribe(taskData => {
      if("taskDefinition" in taskData) {
        this.service["taskDefinition"] = taskData["taskDefinition"]
        this.logsInput["containers"] = [{ "id": "", "name": "Select Container" }]
        if(!("selectedContainer" in this.logsInput)) {
          this.logsInput["selectedContainer"] = this.logsInput["containers"][0]
        }
        this.service["taskDefinition"]["containerDefinitions"].forEach((container, index) => {
          this.logsInput["containers"].push({ "id": container["name"], "name": container["name"] })
        })
      }
      this.sds.describeTasks().subscribe(data => {
        this.loading = false
        this.logsInput["taskArns"] = [{ "id": "", "name": "Select Task" }]
        data["tasks"].forEach((task, index) => {
          let s = task["taskArn"].split("/")
          let startedBy
          if(task["startedBy"].substring(0, 3) == "ecs") {
            startedBy = "ecs"
          } else {
            let b = task["startedBy"].split("-")
            startedBy = b[0]
          }
          let startedAt = moment(task["startedAt"])
          let n = s[1] + " (" + task["lastStatus"] + ")"
          if(task["lastStatus"] == "PENDING") {
            n = n + ", started by " + startedBy
          }  else {
            n = n + ", started " + startedAt.fromNow() + " by " + startedBy
          }
          this.logsInput["taskArns"].push(
            { 
              "id": s[1],
              "name": n,
            }
          )
        })
        if(!("selectedTaskArn" in this.logsInput)) {
          this.logsInput["selectedTaskArn"] = this.logsInput["taskArns"][0]
        }
      })
    });
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
      });
    }
  }
  runTask(): void {
    let valid = false
    let runTaskData = {
      "containerOverrides": []
    }
    let enabledContainers = []
    this.service["taskDefinition"]["containerDefinitions"].forEach((v, i) => {
      let containerName = v.name
      let container = this.runTaskInput[containerName]
      if(container["enabled"]) {
        if("containerCommand" in container) {
          valid = true
          enabledContainers.push(containerName)
        }
        if(container["environmentVariables"]) {
          runTaskData["containerOverrides"].push({
            "name": containerName,
            "command": ["bash", "-c", "eval $(aws-env) && " + container["containerCommand"]]
          })
        } else {
          runTaskData["containerOverrides"].push({
            "name": containerName,
            "command": ["sh", "-c", container["containerCommand"]]
          })
        }
      } else {
        // check if essential, otherwise sleep until timeout
        if(v.essential) {
          runTaskData["containerOverrides"].push({
            "name": containerName,
            "command": ["sh", "-c", "echo 'Container disabled' && sleep "+(this.runTaskConfig.maxExecutionTime+60) ]
          })
        } else {
          runTaskData["containerOverrides"].push({
            "name": containerName,
            "command": ["sh", "-c", "echo 'Container disabled'"]
          })
        }
      }
    })
    if(valid) {
      this.saving = true
      this.sds.runTask(runTaskData).subscribe(data => {
        if("taskArn" in data) {
          //console.log("Taskarn: ", data["taskArn"])
        } else {
          this.alertService.error(data["error"]);
        }
        let t = data["taskArn"].split("/")
        this.saving = false
        this.logsInput["selectedContainer"] = { "id": enabledContainers[0], "name": enabledContainers[0] }
        this.logsInput["selectedTaskArn"] =  { "id": t[1] }
        this.onClickLogs(true)
        this.updateLogs()
      });
    } else {
      this.alertService.error("Invalid task configuration")
    }
  }
  updateLogs(): void {
    if(this.logsInput["selectedContainer"]["id"] == "" || this.logsInput["selectedTaskArn"]["id"] == "" || this.logsInput["selectedTimerange"]["id"] == "") {
      return
    }
    let start
    switch(this.logsInput["selectedTimerange"]["id"]) {
      case "last-7d": {
        start = moment().subtract(7, 'days').toISOString()
      }
      case "last-14d": {
        start = moment().subtract(14, 'days').toISOString()
      }
      case "last-30d": {
        start = moment().subtract(30, 'days').toISOString()
      }
      case "last-1y": {
        start = moment().subtract(1, 'year').toISOString()
      }
      default: {
        start = moment().subtract(1, 'day').toISOString()
      }
    }
    let params = {
      "containerName": this.logsInput["selectedContainer"]["id"],
      "taskArn": this.logsInput["selectedTaskArn"]["id"],
      "start": start,
      "end": moment().toISOString(),
    }
    this.loadingLogs = true
    delete this.service["logs"]
    this.sds.getServiceLog(params).subscribe(data => {
      this.loadingLogs = false
      if("error" in data) {
        let errorMsg: string = data["error"]
        if(errorMsg == "") {
          this.alertService.error("Error, but error message was empty");
        } else {
          if(errorMsg.startsWith("ResourceNotFoundException")) {
            this.service["logs"] = { "count": 0 }
          } else {
            this.alertService.error(errorMsg);
          }
        }
        return
      }
      this.service["logs"] = data["logs"]
      if(!this.service["logs"]["logEvents"]) {
        this.service["logs"]["count"] = 0
      } else {
        this.service["logs"]["count"] = this.service["logs"]["logEvents"].length
      }
    });
  }
  refreshLogs(): void {
    this.onClickLogs(false)
    this.updateLogs()
  }
  compareByID(v1, v2) {
    return v1 && v2 && v1["id"] == v2["id"];
  }

  newAutoscalingPolicy() {
    this.scalingInput.newAutoscalingPolicy = true
    this.scalingInputPolicy.metric = "cpu"
    this.scalingInputPolicy.comparisonOperator = "greaterThanOrEqualToThreshold"
    this.scalingInputPolicy.thresholdStatistic = "average"
    this.scalingInputPolicy.evaluationPeriods = "3"
    this.scalingInputPolicy.period = "60"
    this.scalingInputPolicy.scalingAdjustment = "1"
  }

  addAutoscalingPolicy() {
    this.loading = true
    this.scalingInput.serviceName = this.service.serviceName
    this.scalingInput.minimumCount = Number(this.scalingInput.minimumCount) || 0
    this.scalingInput.desiredCount = Number(this.scalingInput.desiredCount) || 0
    this.scalingInput.maximumCount = Number(this.scalingInput.maximumCount) || 0
    this.scalingInput.policies = []
    this.scalingInput.policies[0] = this.scalingInputPolicy
    this.scalingInput.policies[0].datapointsToAlarm = Number(this.scalingInputPolicy.datapointsToAlarm) || 0
    this.scalingInput.policies[0].evaluationPeriods = Number(this.scalingInputPolicy.datapointsToAlarm) || 0 // same evaluationperiods as datapointsToAlarm
    this.scalingInput.policies[0].period = Number(this.scalingInputPolicy.period)
    this.scalingInput.policies[0].scalingAdjustment = Number(this.scalingInputPolicy.scalingAdjustment)
    this.scalingInput.policies[0].threshold = Number(this.scalingInputPolicy.threshold) || 0
    this.sds.putAutoscaling(this.scalingInput).subscribe(
      data => {
        this.loading = false
        this.scalingInput.newAutoscalingPolicy = false
        this.onClickScaling()
      },
      err => {
        this.loading = false
        this.alertService.error(err["error"]["error"])
      }
    );
  }
  saveAutoscalingPolicy() {
    this.loading = true
    let putData = this.scalingInput
    putData.serviceName = this.service.serviceName
    putData.minimumCount = Number(this.scalingInput.minimumCount) || 0
    putData.desiredCount = Number(this.scalingInput.desiredCount) || 0
    putData.maximumCount = Number(this.scalingInput.maximumCount) || 0
    putData.policies = []
    this.sds.putAutoscaling(putData).subscribe(
      data => {
        this.loading = false
        this.scalingInput.newAutoscalingPolicy = false
        console.log(data)
        this.alertService.success("Succesfully applied autoscaling settings", 5)
      },
      err => {
        this.loading = false
        this.alertService.error(err["error"]["error"])
      }
    );
  }
  
  enableAutoscaling() {
    this.scalingInput.autoscaling = true
  }
  deletingAutoscalingPolicy(loading) {
    if(loading) {
      this.loading = loading
    }
  }
  
  deletingItem(loading) {
    if(loading) {
      this.loading = loading
    }
  }
  deletedItem(data) {
    if(data.action == 'deleteParameter') {
      let selectedParameter = data.selectedItem
      this.loading = true
      delete this.parameters["map"][selectedParameter]
      this.parameters["keys"] = []
      for (let key in this.parameters["map"]) {
        this.parameters["keys"].push(key)
      }
      this.loading = false
    } else if(data.action == 'deleteAutoscalingPolicy') {
      let selectedAutoscalingPolicy = data.selectedItem
      this.loading = true
      let index = -1
      this.scalingInput.policies.forEach((policy, i) => {
        if(policy.policyName == selectedAutoscalingPolicy) {
          index = i
        }
      })
      if (index > -1) {
         this.scalingInput.policies.splice(index, 1);
      }
      this.scalingInput.policyCount--
      this.loading = false
    } else if(data.action == 'disableAutoscaling') {
      this.scalingInput.autoscaling = false
      this.scalingInput.newAutoscalingPolicy = false
      this.scalingInput.policies = []
      this.scalingInput.policyCount = 0
      this.loading = false
      this.alertService.success("Succesfully disabled autoscaling", 5)
    }
  }
}
